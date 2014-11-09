package main

import (
	"fmt"
	"github.com/alexaandru/utils"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/s3"
	"sync"
)

// Exit codes
const (
	Success = iota
	CachingFailed
	LoadingCacheFailed
	S3Failed
	S3AuthError
	FileReadFailed
	CmdLineOptionError
	BadHeaderDefinition
)

type uploader func(aws.Auth, *s3.Bucket, *sourceFile) error

// filesLists returns both the current files list as well as the difference from the old (cached) files list.
func filesLists() (current utils.FileHashes, diff []string) {
	current = utils.FileHashesNew(opts.source)
	old := utils.FileHashes{}
	old.Load(opts.cacheFile)

	diff = current.Diff(old)

	return
}

// s3put puts one file to S3 and tries to recover from some errors.
func s3put(auth aws.Auth, bucket *s3.Bucket, src *sourceFile) (err error) {
	if err = bucket.PutHeader(src.fname, src.body(), src.hdrs, s3.PublicRead); err != nil {
		// FIXME: Implement exponential backoff.
		if isRecoverable(err) {
			fmt.Print(msg("Warn: upload failed, retrying: "+src.fname, "r", "Retry: "+src.fname))
			bucket = s3.New(auth, aws.EUWest).Bucket(opts.bucketName)
			return s3put(auth, bucket, src)
		}
		quit("S3 error:", err, S3Failed)
	}
	fmt.Print(msg("Uploaded "+src.fname, "."))

	return
}

// FIXME: On unrecoverable error, it should still cache the successfull uploads.
func main() {
	validateCmdLineFlags(opts)

	auth, err := aws.EnvAuth()
	if err != nil {
		quit("S3 Error:", err, S3AuthError)
	}

	current, diff := filesLists()
	bucket, wg, uploads := s3.New(auth, aws.EUWest).Bucket(opts.bucketName), new(sync.WaitGroup), make(chan *sourceFile)
	if len(diff) == 0 {
		fmt.Print(msg("Nothing to upload."))
		goto Finish
	}
	fmt.Print(msg(fmt.Sprintf("There are %d files to be uploaded to '%s'", len(diff), opts.bucketName), "Uploading "))

	goto Upload // noop, just so that we can have an Upload: label

Upload:
	if !opts.doUpload {
		fmt.Print(msg("Skipping upload"))
		goto Cache
	}

	wg.Add(opts.workersCount)
	for i := 0; i < opts.workersCount; i++ {
		go func(fn uploader) {
			defer wg.Done()
			for src := range uploads {
				if opts.dryRun {
					fmt.Print(msg("Pretending to upload "+src.fname, "."))
					continue
				}
				fn(auth, bucket, src)
			}
		}(opts.upload)
	}

	for _, fname := range diff {
		uploads <- newSourceFile(fname)
	}

	close(uploads)
	wg.Wait()
	fmt.Print(msg("Uploaded files.", "\n"))

Cache:
	if !opts.doCache {
		fmt.Print(msg("Skipping cache.", ""))
		goto Finish
	}

	if opts.dryRun {
		fmt.Print(msg("Pretending to update cache.\nUpdated cache.", ""))
		goto Finish
	}

	if err := current.Dump(opts.cacheFile); err != nil {
		quit("Caching failed: ", err, CachingFailed)
	}
	fmt.Print(msg("Updated cache.", ""))

Finish:
	fmt.Print(msg("All done!"))
}
