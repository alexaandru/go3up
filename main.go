package main

import (
	"fmt"
	"math"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/alexaandru/utils"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/s3"
)

// Exit codes
const (
	Success = iota
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
	current = utils.FileHashesNew(opts.source)
	old := utils.FileHashes{}
	old.Load(opts.cacheFile)
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
			say("Failed to upload "+src.fname+": "+err.Error(), "f")
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

// Generate an S3 put func. It holds the bucket in a closure.
// FIXME: For some (all?) errors, we should re-initialize the bucket before retrying.
func s3putGen() (up uploader, err error) {
	auth, err := aws.EnvAuth()
	if err != nil {
		return
	}

	bucket := s3.New(auth, aws.EUWest).Bucket(opts.bucketName)
	return func(src *sourceFile) (err error) {
		var body []byte
		body, err = src.body()
		if err != nil {
			// If we can't read the file, there's no point to retry.
			src.attempts = maxTries
			return
		}

		return bucket.PutHeader(src.fname, body, src.hdrs, s3.PublicRead)
	}, nil
}

func main() {
	if err := validateCmdLineFlags(opts); err != nil {
		fmt.Printf("Commandline flags error: %s. Please use 'go3up -h' for help.\n", err)
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
	say(fmt.Sprintf("There are %d files to be uploaded to '%s'", len(diff), opts.bucketName), "Uploading ")

	if !opts.doUpload {
		say("Skipping upload")
		goto Cache
	}

	wgUploads.Add(len(diff))
	wgWorkers.Add(opts.workersCount)
	for i := 0; i < opts.workersCount; i++ {
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
		goto Finish
	}

	if opts.dryRun {
		say("Pretending to update cache.")
		goto Finish
	}

	current = current.Reject(rejected.list)
	if err := current.Dump(opts.cacheFile); err != nil {
		fmt.Println("Caching failed: ", err)
		os.Exit(CachingFailure)
	}
	say("Done updating cache.")

Finish:
	say("All done!", " done!\n")
}
