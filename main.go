package main

import (
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/alexaandru/utils"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// Exit codes
const (
	Success = iota
	SetupFailed
	S3AuthError
	CmdLineOptionError
	CachingFailure
)

// max number of attempts to retry a failed upload.
const maxTries = 10

// signature of an s3 uploader func
type uploader func(*sourceFile) error

// filesLists returns both the current files list as well as the difference from the old (cached) files list.
func filesLists() (current utils.FileHashes, diff []string) {
	current = utils.FileHashesNew(opts.Source)
	old := utils.FileHashes{}
	old.Load(opts.CacheFile)
	diff = current.Diff(old)

	return
}

// upload fetches sourceFiles from uploads chan, attempts to upload them and enqueue the results to
// completed list. On failure it attempts to retry, up to maxTries per source file.
func upload(id string, fn uploader, uploads chan *sourceFile, rejected *syncedlist, wgUploads, wgWorkers *sync.WaitGroup) {
	defer wgWorkers.Done()

	for src := range uploads {
		src := src

		if opts.dryRun {
			say("Pretending to upload "+src.fname, ".")
			wgUploads.Done()
			continue
		}

		err := fn(src)
		if err == nil {
			wgUploads.Done()
			say("Uploaded "+src.fname, ".")
			continue
		}

		src.recordAttempt()
		if !src.retriable() || !isRecoverable(err) {
			rejected.add(src.fname)
			say("Failed to upload "+src.fname+": "+err.Error(), "F")
			wgUploads.Done()
			continue
		}

		go func() {
			say("Retrying "+src.fname, "r")
			wait := time.Duration(100.0*math.Pow(2, float64(src.attempts))) * time.Millisecond
			if appEnv == "test" {
				wait = time.Nanosecond
			}
			<-time.After(wait)
			uploads <- src
		}()
	}
}

// Generate an S3 upload func. It holds the bucket in a closure.
func s3putGen() (up uploader, err error) {
	if appEnv == "test" {
		return func(src *sourceFile) error {
			// TODO: capture the sourceFile for testing
			return nil
		}, nil
	}

	return func(src *sourceFile) (err error) {
		f, err := os.Open(filepath.Join(opts.Source, src.fname))
		if err != nil {
			return err
		}

		var r io.Reader = f
		cacheControl, contentEnc, contentType, sse := src.getHeader(CacheControl), src.getHeader(ContentEncoding),
			betterMime(src.fname), src.getHeader(Encryption)
		if src.gzip {
			rr, w := io.Pipe()
			wz := gzip.NewWriter(w)
			go func() {
				// FIXME: We need a better way to handle these.
				if _, err2 := io.Copy(wz, f); err2 != nil {
					panic(fmt.Errorf("decryption error: %v", err2))
				}
				if err2 := wz.Close(); err2 != nil {
					panic(fmt.Errorf("decryption error: %v", err2))
				}
				if err2 := w.Close(); err2 != nil {
					panic(fmt.Errorf("decryption error: %v", err2))
				}
			}()

			r = rr
		}

		u := s3manager.NewUploader(sess, func(opts *s3manager.Uploader) {
			opts.S3 = s3svc
			opts.LeavePartsOnError = false
		})
		_, err = u.Upload(&s3manager.UploadInput{
			Key:                  &src.fname,
			Body:                 r,
			Bucket:               &opts.BucketName,
			ContentType:          &contentType,
			ContentEncoding:      contentEnc,
			CacheControl:         cacheControl,
			ServerSideEncryption: sse,
		})

		return err
	}, nil
}

func main() {
	if err := validateCmdLineFlags(opts); err != nil {
		fmt.Printf("Required field missing: %v.\n\nUsage:\n", err)
		flag.PrintDefaults()
		os.Exit(CmdLineOptionError)
	}

	s3put, err := s3putGen()
	if err != nil {
		fmt.Println("S3 Error:", err)
		os.Exit(S3AuthError)
	}

	uploads, rejected := make(chan *sourceFile), &syncedlist{}
	wgUploads, wgWorkers := new(sync.WaitGroup), new(sync.WaitGroup)

	current, diff := filesLists()
	if len(diff) == 0 {
		say("Nothing to upload.", "Nothing to upload.\n")
		os.Exit(Success)
	}
	say(fmt.Sprintf("There are %d files to be uploaded to '%s'", len(diff), opts.BucketName), "Uploading ")

	if !opts.doUpload {
		say("Skipping upload")
		goto Cache
	}

	wgUploads.Add(len(diff))
	wgWorkers.Add(opts.WorkersCount)
	for i := 0; i < opts.WorkersCount; i++ {
		go upload(fmt.Sprintf("%d", i), s3put, uploads, rejected, wgUploads, wgWorkers)
	}

	sort.Strings(diff)
	for _, fname := range diff {
		uploads <- newSourceFile(fname)
	}

	wgUploads.Wait()
	close(uploads)
	wgWorkers.Wait()
	say("Done uploading files.")

Cache:
	if !opts.doCache {
		say("Skipping cache.")
		goto Done
	}

	if opts.dryRun {
		say("Pretending to update cache.")
		goto Done
	}

	current = current.Reject(rejected.list)
	if err := current.Dump(opts.CacheFile); err != nil {
		fmt.Println("Caching failed: ", err)
		os.Exit(CachingFailure)
	}
	say("Done updating cache.")

Done:
	say("All done!", " done!\n")
}
