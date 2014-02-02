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
	"sync"
)

// RequestTimeoutError is a frequent error received when there are too many threads.
// If you encounter this too often, reduce the number of workers.
const RequestTimeoutError = "Your socket connection to the server was not read from or written to within the timeout period. Idle connections will be closed."

// Various exit codes
const (
	Success = iota
	CachingFailed
	LoadingCacheFailed
	S3Failed
	S3AuthError
	FileReadFailed
	CmdLineOptionError
)

type options struct {
	workersCount int
	bucketName, source,
	cacheFile string
	dryRun, verbose, quiet, doCache, doUpload bool
}

var auth aws.Auth

// s3put puts one file to S3 and tries to recover from RequestTimeoutError
func s3put(bucket *s3.Bucket, fname string, opts *options) (err error) {
	fullName := filepath.Join(opts.source, fname)
	if data, err := ioutil.ReadFile(fullName); err == nil {
		if err = bucket.Put(fname, data, betterMime(fname), s3.PublicRead); err != nil {
			if err.Error() == RequestTimeoutError {
				if opts.verbose || opts.quiet {
					fmt.Println("Warn: upload failed, retrying:", fname)
				} else {
					fmt.Print("r")
				}
				bucket = s3.New(auth, aws.EUWest).Bucket(opts.bucketName)
				return s3put(bucket, fname, opts)
			}

			fmt.Println("S3 error:", err)
			os.Exit(S3Failed)
		} else {
			if opts.verbose {
				fmt.Println("Uploaded", fname)
			} else if !opts.quiet {
				fmt.Print(".")
			}
		}
	} else {
		fmt.Println("Read error:", err)
		os.Exit(FileReadFailed)
	}

	return
}

func uploadWorker(bucket *s3.Bucket, linkChan chan string, wg *sync.WaitGroup, opts *options) {
	defer wg.Done()

	for fname := range linkChan {
		if opts.dryRun {
			if opts.verbose {
				fmt.Println("Pretending to upload", fname)
			} else if !opts.quiet {
				fmt.Print(".")
			}
		} else {
			s3put(bucket, fname, opts)
		}
	}
}

func prepareUploadList(opts *options) (current utils.FileHashes, diff []string) {
	var old utils.FileHashes
	current, old = utils.FileHashesNew(opts.source), utils.FileHashes{}
	old.Load(opts.cacheFile)
	diff = current.Diff(old)

	if len(diff) == 0 {
		if !opts.quiet {
			fmt.Println("Nothing to upload.")
		}
		os.Exit(Success)
	} else if opts.verbose {
		fmt.Printf("List of changes ready, there are %d files to be uploaded to %s:\n", len(diff), opts.bucketName)
	}

	return
}

func upload(diff []string, opts *options) {
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
		go uploadWorker(bucket, comm, wg, opts)
	}

	for _, fname := range diff {
		comm <- fname
	}
}

func handleUpload(diff []string, opts *options) {
	if opts.doUpload {
		if !(opts.quiet || opts.verbose) {
			fmt.Printf("Uploading to %s ", opts.bucketName)
		}

		upload(diff, opts)

		if !opts.quiet {
			fmt.Println("done!")
		}
	} else if opts.verbose {
		fmt.Println("Skipping upload")
	}
}

func handleCache(current utils.FileHashes, opts *options) {
	if opts.doCache {
		if !opts.dryRun {
			if err := current.Dump(opts.cacheFile); err != nil {
				fmt.Println(err)
				os.Exit(CachingFailed)
			}
			if opts.verbose {
				fmt.Println("Updated cache. All done!")
			}
		} else if opts.verbose {
			fmt.Println("Pretending to update cache. All done!")
		}
	} else if opts.verbose {
		fmt.Println("Skipping cache")
	}
}

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

func validateCmdLineFlags(opts *options) (err error) {
	// FIXME: More checks: if source is a folder, if hash file is a file, if bucket exists, etc.
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

// NOTE: currently it is limited to just checking paths (if set and if they exist)
func validateCmdLineFlag(label, val string) (err error) {
	switch label {
	case "Bucket Name":
		if val == "" {
			err = errors.New(label + " is not set")
		}
		return
	default:
		_, err = os.Stat(val)
		return

	}
}

func betterMime(fname string) (mt string) {
	ext := filepath.Ext(fname)
	if mt = mime.TypeByExtension(ext); mt != "" {
		return
	} else if ext == ".JPG" {
		mt = "image/jpeg"
	} else if ext == ".ttf" {
		mt = "binary/octet-stream"
	}

	return
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	opts := processCmdLineFlags()
	current, diff := prepareUploadList(opts)
	handleUpload(diff, opts)
	handleCache(current, opts)
}
