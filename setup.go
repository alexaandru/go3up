package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
)

type options struct {
	upload       uploader
	workersCount int
	bucketName, source,
	cacheFile string
	dryRun, verbose, quiet, doCache, doUpload, gzipHTML bool
}

var opts *options

// Order matters: first hit, first served.
// TODO: Make this configurable somehow, so that end users can provide their own mappings.
var r = regexp.MustCompile
var customHeadersDef = []pathToHeaders{
	pathToHeaders{r("index\\.html"),
		headersDef{ContentEncoding: "gzip", CacheControl: "max-age=1800"}},

	pathToHeaders{r("[^/]*\\.html$"),
		headersDef{ContentEncoding: "gzip", CacheControl: "max-age=3600"}},

	pathToHeaders{r("\\.html$"),
		headersDef{ContentEncoding: "gzip", CacheControl: "max-age=86400"}},

	pathToHeaders{r("\\.xml$"),
		headersDef{ContentEncoding: "gzip", CacheControl: "max-age=1800"}},

	pathToHeaders{r("\\.ico$"),
		headersDef{ContentEncoding: "gzip", CacheControl: "max-age=31536000"}},

	pathToHeaders{r("\\.(js|css)$"),
		headersDef{ContentEncoding: "gzip", CacheControl: "max-age=31536000"}},

	pathToHeaders{r("images/articole/.*(jpg|JPG|png|PNG)$"),
		headersDef{CacheControl: "max-age=31536000"}},

	pathToHeaders{r("\\.(jpg|JPG|png|PNG)$"),
		headersDef{CacheControl: "max-age=31536000"}},
}

// processCmdLineFlags wraps the command line flags handling.
func processCmdLineFlags(opts *options) {
	flag.IntVar(&opts.workersCount, "workers", 42, "No. of workers/threads to use for S3 uploads")
	flag.StringVar(&opts.bucketName, "bucket", "", "S3 bucket to upload files to")
	flag.StringVar(&opts.source, "source", "output", "Source folder for files to be uploaded to S3")
	flag.StringVar(&opts.cacheFile, "cachefile", filepath.Join(".go3up.txt"), "Location of the cache file")
	flag.BoolVar(&opts.dryRun, "dry", false, "Dry run (no upload/cache update)")
	flag.BoolVar(&opts.verbose, "verbose", false, "Print the name of the files as they are uploaded")
	flag.BoolVar(&opts.quiet, "quiet", false, "Print only warnings and errors")
	flag.BoolVar(&opts.doUpload, "upload", true, "Do perform an upload")
	flag.BoolVar(&opts.doCache, "cache", true, "Do update the cache")
	flag.BoolVar(&opts.gzipHTML, "gzip", true, "Gzip HTML files")
	flag.Parse()
}

// validateCmdLineFlags validates some of the flags, mostly paths. Defers actual validation to validateCmdLineFlag()
func validateCmdLineFlags(opts *options) (err error) {
	flags := map[string]string{"Bucket Name": opts.bucketName, "Source": opts.source, "Cache file": opts.cacheFile}
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

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	opts = new(options)
	processCmdLineFlags(opts)
	opts.upload = s3put
}