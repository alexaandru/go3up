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
)

// max number of attempts to retry a failed upload.
const maxTries = 10

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
func upload(id string, fn uploader, uploads chan *sourceFile, completed *completedList, wgUploads, wgWorkers *sync.WaitGroup) {
	defer wgWorkers.Done()

	for src := range uploads {
		src := src

		if opts.dryRun {
			fmt.Print(msg("Pretending to upload "+src.fname, "."))
			wgUploads.Done()
			continue
		}

		err := fn(src)
		if err == nil {
			completed.add(src.fname)
			wgUploads.Done()
			fmt.Print(msg("Uploaded "+src.fname, "."))
			continue
		}

		src.recordAttempt()
		if !src.retriable() || !isRecoverable(err) {
			fmt.Print(msg("Failed to upload "+src.fname, "f"))
			wgUploads.Done()
			continue
		}

		go func() {
			wait := time.Duration(100.0*math.Pow(2, float64(src.attempts))) * time.Millisecond
			if appEnv == "test" {
				wait = time.Nanosecond
			}
			<-time.After(wait)
			uploads <- src
		}()
	}
}

// FIXME: For some (all?) errors, we should re-initialize the bucket before retrying.
func main() {
	validateCmdLineFlags(opts)
	goto Setup

Setup:
	// AWS setup
	auth, err := aws.EnvAuth()
	if err != nil {
		fmt.Println("S3 Error:", err)
		os.Exit(S3AuthError)
	}
	newBucket := func() *s3.Bucket { return s3.New(auth, aws.EUWest).Bucket(opts.bucketName) }
	bucket := newBucket()
	s3put := func(src *sourceFile) (err error) {
		var body []byte
		body, err = src.body()
		if err != nil {
			// If we can't read the file, there's no point to retry.
			src.attempts = maxTries
			return
		}

		return bucket.PutHeader(src.fname, body, src.hdrs, s3.PublicRead)
	}

	// Sync setup
	uploads, completed := make(chan *sourceFile), &completedList{}
	wgUploads, wgWorkers := new(sync.WaitGroup), new(sync.WaitGroup)

	// Current list of files, and diff to be uploaded
	current, diff := filesLists()
	if len(diff) == 0 {
		fmt.Print(msg("Nothing to upload."))
		goto Finish
	}
	fmt.Print(msg(fmt.Sprintf("There are %d files to be uploaded to '%s'", len(diff), opts.bucketName), "Uploading "))
	goto Upload

Upload:
	if !opts.doUpload {
		fmt.Print(msg("Skipping upload", ""))
		goto Cache
	}

	// We need to wait until ALL files are processed, one way or another.
	wgUploads.Add(len(diff))
	wgWorkers.Add(opts.workersCount)
	for i := 0; i < opts.workersCount; i++ {
		go upload(fmt.Sprintf("%d", i), s3put, uploads, completed, wgUploads, wgWorkers)
	}

	sort.Strings(diff)
	for _, fname := range diff {
		uploads <- newSourceFile(fname)
	}

	wgUploads.Wait()
	close(uploads)
	wgWorkers.Wait()
	fmt.Print(msg("Done uploading files.", ""))

Cache:
	if !opts.doCache {
		fmt.Print(msg("Skipping cache.", ""))
		goto Finish
	}

	if opts.dryRun {
		fmt.Print(msg("Pretending to update cache.\nUpdated cache.", ""))
		goto Finish
	}

	// Only retain files actually completed.
	current = current.Filter(completed.list)
	if err := current.Dump(opts.cacheFile); err != nil {
		fmt.Println("Caching failed: ", err)
		goto Finish
	}
	fmt.Print(msg("Done updating cache.", ""))

Finish:
	fmt.Print(msg("All done!", " done!\n"))
}
