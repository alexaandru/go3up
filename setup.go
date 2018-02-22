package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"runtime"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

var opts = &options{
	WorkersCount: runtime.NumCPU() * 2,
	Source:       "output",
	CacheFile:    ".go3up.txt",
	doUpload:     true,
	doCache:      true,
	Region:       os.Getenv("AWS_DEFAULT_REGION"),
	Profile:      os.Getenv("AWS_DEFAULT_PROFILE"),
	cfgFile:      ".go3up.json",
}

var appEnv string

// s3 session.
var sess = session.New()

var s3svc *s3.S3

var say func(...string)

// Order matters: first hit, first served.
// TODO: Make this configurable somehow, so that end users can provide their own mappings.
var r = regexp.MustCompile
var customHeadersDef = []pathToHeaders{
	{r("index\\.html"), headers{ContentEncoding: "gzip", CacheControl: "max-age=1800"}},       // 1800
	{r("articole.*\\.html$"), headers{ContentEncoding: "gzip", CacheControl: "max-age=3600"}}, // 86400
	{r("[^/]*\\.html$"), headers{ContentEncoding: "gzip", CacheControl: "max-age=3600"}},
	{r("\\.xml$"), headers{ContentEncoding: "gzip", CacheControl: "max-age=1800"}},
	{r("\\.ico$"), headers{ContentEncoding: "gzip", CacheControl: "max-age=31536000"}},
	{r("\\.(js|css)$"), headers{ContentEncoding: "gzip", CacheControl: "max-age=31536000"}},
	{r("images/articole/.*(jpg|JPG|png|PNG)$"), headers{CacheControl: "max-age=31536000"}},
	{r("\\.(jpg|JPG|png|PNG)$"), headers{CacheControl: "max-age=31536000"}},
}

// processCmdLineFlags wraps the command line flags handling.
func processCmdLineFlags(opts *options) {
	flag.IntVar(&opts.WorkersCount, "workers", opts.WorkersCount, "No. of workers to use for uploads")
	flag.StringVar(&opts.BucketName, "bucket", opts.BucketName, "Bucket to upload files to")
	flag.StringVar(&opts.Source, "source", opts.Source, "Source folder for files to be uploaded")
	flag.StringVar(&opts.CacheFile, "cachefile", opts.CacheFile, "Location of the cache file")
	flag.StringVar(&opts.Region, "region", opts.Region, "AWS region")
	flag.StringVar(&opts.Profile, "profile", opts.Profile, "AWS shared profile")
	flag.StringVar(&opts.cfgFile, "cfgfile", opts.cfgFile, "Config file location")
	flag.BoolVar(&opts.dryRun, "dry", opts.dryRun, "Dry run (do not upload/update cache)")
	flag.BoolVar(&opts.verbose, "verbose", opts.verbose, "Print the name of the files as they are uploaded")
	flag.BoolVar(&opts.quiet, "quiet", opts.quiet, "Print only warnings and/or errors")
	flag.BoolVar(&opts.doUpload, "upload", opts.doUpload, "Do perform an upload")
	flag.BoolVar(&opts.doCache, "cache", opts.doCache, "Do update the cache")
	flag.BoolVar(&opts.Encrypt, "encrypt", opts.Encrypt, "Encrypt files on server side")
	flag.BoolVar(&opts.saveCfg, "save", opts.saveCfg, "Saves the current commandline options to a config file")
	flag.Parse()
}

// validateCmdLineFlags validates some of the flags, mostly paths. Defers actual validation to validateCmdLineFlag()
func validateCmdLineFlags(opts *options) (err error) {
	flags := map[string]string{
		"Bucket Name": opts.BucketName,
		"Source":      opts.Source,
		"Cache file":  opts.CacheFile,
	}
	for label, val := range flags {
		if err = validateCmdLineFlag(label, val); err != nil {
			return
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

func initAWSClient() {
	creds := credentials.NewChainCredentials(
		[]credentials.Provider{
			&credentials.SharedCredentialsProvider{Profile: opts.Profile},
			&ec2rolecreds.EC2RoleProvider{Client: ec2metadata.New(sess)},
			&credentials.EnvProvider{},
		})

	retries := 2
	awsCfg := &aws.Config{
		Credentials: creds,
		Region:      &opts.Region,
		MaxRetries:  &retries,
	}

	defer func() {
		if r := recover(); r != nil {
			// If we got to this, then getting the credentials failed - nothing else
			// can raise a panic in here.
			abort(fmt.Errorf("Unable to initialize AWS credentials - please check environment."))
		}
	}()
	if _, err := creds.Get(); err != nil {
		abort(err)
	}

	s3svc = s3.New(sess, awsCfg)
}

func abort(msg error) {
	say(msg.Error())
	os.Exit(SetupFailed)
}

func init() {
	oldCfgFile := opts.cfgFile
	if err := opts.restore(opts.cfgFile); err != nil {
		abort(err)
	}
	processCmdLineFlags(opts)
	if opts.cfgFile != oldCfgFile { // we were given a different config file, use that instead.
		if err := opts.restore(opts.cfgFile); err != nil {
			abort(err)
		}
	}
	if opts.saveCfg {
		if err := opts.dump(opts.cfgFile); err != nil {
			abort(err)
		}
	}
	appEnv = "production"
	say = loggerGen()
	initAWSClient()
}
