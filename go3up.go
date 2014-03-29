/*
Go3Up (Go S3 Uploader) is a small S3 uploader tool.

It was created in order to speed up S3 uploads by employing a local caching of files md5 sums.
That way, on subsequent runs, go3up can compute a list of the files that changed since the
last upload and limit the upload only to those.

The initial use case was a large static site (with 10k+ files) that frequently changed only
a small subset of files (about ~100 routinely). In that particular case, the time reduction by
switching from s3cmd to go3up was significant.

On uploads with empty cache there may not be any benefit.

The current focus of the tool is just one way/uploads (without deleting things that were removed
locally, yet). That may (or not) change in the future.
*/
package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/alexaandru/utils"
	"io/ioutil"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/s3"
	"mime"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// Command line options/flags.
type options struct {
	workersCount int
	bucketName, source,
	cacheFile string
	dryRun, verbose, quiet, doCache, doUpload bool
}

// Exit codes.
const (
	Success = iota
	CachingFailed
	LoadingCacheFailed
	S3Failed
	S3AuthError
	FileReadFailed
	CmdLineOptionError
)

// S3 errors that we will retry.
var recoverableErrorsSuffixes = []string{
	"Idle connections will be closed.",
	"EOF",
	"broken pipe",
}

// recoverable verifies if the error given is in recoverableErrorsSuffixes list.
func recoverable(err error) bool {
	for _, errSuffix := range recoverableErrorsSuffixes {
		if strings.HasSuffix(err.Error(), errSuffix) {
			return true
		}
	}

	return false
}

// quit aborts the program with exitCode, after it prints the error message.
func quit(msg string, err error, exitCode int) {
	fmt.Println(msg, err)
	os.Exit(exitCode)
}

// display prints either s1 or s2, conditionally.
func display(cond bool, s1 string, rest ...string) {
	if cond {
		fmt.Println(s1)
	} else if len(rest) > 0 {
		fmt.Print(rest[0])
	}
}

// s3put puts one file to S3 and tries to recover from some errors.
func s3put(auth aws.Auth, bucket *s3.Bucket, fname string, opts *options) (err error) {
	fullName := filepath.Join(opts.source, fname)
	if data, err := ioutil.ReadFile(fullName); err == nil {
		if err = bucket.Put(fname, data, betterMime(fname), s3.PublicRead); err != nil {
			if recoverable(err) { // FIXME: Implement exponential backoff.
				display(opts.verbose || opts.quiet, "Warn: upload failed, retrying: "+fname, "r")
				bucket = s3.New(auth, aws.EUWest).Bucket(opts.bucketName)
				return s3put(auth, bucket, fname, opts)
			}
			quit("S3 error:", err, S3Failed)
		} else {
			if !opts.quiet {
				display(opts.verbose, "Uploaded "+fname, ".")
			}
		}
	} else {
		quit("Read error:", err, FileReadFailed)
	}

	return
}

// filesLists returns both the current files list as well as the difference from the old
// (cached) files list.
func filesLists(opts *options) (current utils.FileHashes, diff []string) {
	var old utils.FileHashes
	current, old = utils.FileHashesNew(opts.source), utils.FileHashes{}
	old.Load(opts.cacheFile)
	diff = current.Diff(old)

	if len(diff) == 0 {
		display(!opts.quiet, "Nothing to upload.")
		os.Exit(Success)
	} else {
		display(opts.verbose,
			fmt.Sprintf("List of changes ready, there are %d files to be uploaded to %s:\n", len(diff), opts.bucketName))
	}

	return
}

// performUpload handles the actual files upload.
func performUpload(diff []string, opts *options) {
	var auth aws.Auth
	var err error

	if auth, err = aws.EnvAuth(); err != nil {
		fmt.Println(err)
		os.Exit(S3AuthError)
	}

	bucket := s3.New(auth, aws.EUWest).Bucket(opts.bucketName)

	wg := new(sync.WaitGroup)
	defer wg.Wait()
	comm := make(chan string)
	defer close(comm)

	wg.Add(opts.workersCount)
	for i := 0; i < opts.workersCount; i++ {
		go func() {
			defer wg.Done()
			for fname := range comm {
				if opts.dryRun {
					if !opts.quiet {
						display(opts.verbose, "Pretending to upload "+fname, ".")
					}
				} else {
					s3put(auth, bucket, fname, opts)
				}
			}
		}()

	}

	for _, fname := range diff {
		comm <- fname
	}
}

// upload handles the upload of changed files.
func upload(diff []string, opts *options) {
	if opts.doUpload {
		if !(opts.quiet || opts.verbose) {
			fmt.Printf("Uploading to %s ", opts.bucketName)
		}

		performUpload(diff, opts)
		display(!opts.quiet, "done!")
	} else {
		display(opts.verbose, "Skipping upload")
	}
}

// cache handles the caching of the current list of files.
func cache(current utils.FileHashes, opts *options) {
	if opts.doCache {
		if !opts.dryRun {
			if err := current.Dump(opts.cacheFile); err != nil {
				quit("Caching failed: ", err, CachingFailed)
			}
			display(opts.verbose, "Updated cache. All done!")
		} else {
			display(opts.verbose, "Pretending to update cache. All done!")
		}
	} else {
		display(opts.verbose, "Skipping cache")
	}
}

// processCmdLineFlags wraps the command line flags handling.
func processCmdLineFlags() (opts *options) {
	opts = new(options)
	flag.IntVar(&opts.workersCount, "workers", 42, "No. of workers/threads to use for S3 uploads")
	flag.StringVar(&opts.bucketName, "bucket", "", "S3 bucket to upload files to")
	flag.StringVar(&opts.source, "source", "output", "Source folder for files to be uploaded to S3")
	flag.StringVar(&opts.cacheFile, "cachefile", filepath.Join(".go3up.txt"), "Location of the cache file")
	flag.BoolVar(&opts.dryRun, "dry", false, "Dry run (no upload/cache update)")
	flag.BoolVar(&opts.verbose, "verbose", false, "Print the name of the files as they are uploaded")
	flag.BoolVar(&opts.quiet, "quiet", false, "Print only warnings and errors")
	flag.BoolVar(&opts.doUpload, "upload", true, "Do perform an upload")
	flag.BoolVar(&opts.doCache, "cache", true, "Do update the cache")

	flag.Parse()
	validateCmdLineFlags(opts)

	return
}

// validateCmdLineFlags validates some of the flags, mostly paths.
// Defers actual validation to validateCmdLineFlag()
func validateCmdLineFlags(opts *options) (err error) {
	flags := map[string]string{
		"Bucket Name": opts.bucketName,
		"Source":      opts.source,
		"Cache file":  opts.cacheFile,
	}

	for label, val := range flags {
		if err = validateCmdLineFlag(label, val); err != nil {
			fmt.Printf("%s should be set. Please use 'go3up -h' for help.\n", label)
			os.Exit(CmdLineOptionError)
		}
	}

	return
}

// validateCmdLineFlag handles the actual validation of flags.
func validateCmdLineFlag(label, val string) (err error) {
	switch label {
	case "Bucket Name":
		if val == "" {
			return errors.New(label + " is not set")
		}
	default:
		_, err = os.Stat(val)
	}

	return
}

// betterMime wrapps mime.TypeByExtension and tries to handle a few edge cases.
func betterMime(fname string) (mt string) {
	ext := strings.ToLower(filepath.Ext(fname))
	if mt = mime.TypeByExtension(ext); mt != "" {
		return
	} else if ext == ".ttf" {
		mt = "binary/octet-stream"
	}

	return
}

// TODO: On unrecoverable error, still cache the ones successfully uploaded.
func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	opts := processCmdLineFlags()
	current, diff := filesLists(opts)
	upload(diff, opts)
	cache(current, opts)
}
