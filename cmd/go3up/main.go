package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/alexaandru/go3up"
)

// Exit codes.
const (
	Success = iota
	SetupFailed
	S3AuthError
	CmdLineOptionError
	CachingFailure
)

const defaultCfgFile = ".go3up.json"

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, out io.Writer) int {
	cfgFile := defaultCfgFile

	var (
		dry, verbose, quiet, doUpload, doCache, save, encrypt bool
		bucket, source, cacheFile, region, profile            string
		workers                                               int
	)

	fs := flag.NewFlagSet("go3up", flag.ContinueOnError)
	fs.SetOutput(out)

	fs.IntVar(&workers, "workers", 0, "No. of workers to use for uploads")
	fs.StringVar(&bucket, "bucket", "", "Bucket to upload files to")
	fs.StringVar(&source, "source", "", "Source folder for files to be uploaded")
	fs.StringVar(&cacheFile, "cachefile", "", "Location of the cache file")
	fs.StringVar(&region, "region", "", "AWS region")
	fs.StringVar(&profile, "profile", "", "AWS shared profile")
	fs.StringVar(&cfgFile, "cfgfile", cfgFile, "Config file location")
	fs.BoolVar(&dry, "dry", false, "Dry run (do not upload/update cache)")
	fs.BoolVar(&verbose, "verbose", false, "Print the name of the files as they are uploaded")
	fs.BoolVar(&quiet, "quiet", false, "Print only warnings and/or errors")
	fs.BoolVar(&doUpload, "upload", true, "Do perform an upload")
	fs.BoolVar(&doCache, "cache", true, "Do update the cache")
	fs.BoolVar(&encrypt, "encrypt", false, "Encrypt files on server side")
	fs.BoolVar(&save, "save", false, "Saves the current commandline options to a config file")

	if err := fs.Parse(args); err != nil {
		return CmdLineOptionError
	}

	// Explicitly set flags override the config file; the rest of the
	// settings come from the file, then from built-in defaults.
	set := map[string]bool{}

	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	opts := []go3up.Option{
		go3up.WithConfigFile(cfgFile),
		go3up.WithDryRun(dry),
		go3up.WithVerbose(verbose),
		go3up.WithQuiet(quiet),
		go3up.WithUpload(doUpload),
		go3up.WithCache(doCache),
	}

	if set["workers"] {
		opts = append(opts, go3up.WithWorkers(workers))
	}

	if set["bucket"] {
		opts = append(opts, go3up.WithBucket(bucket))
	}

	if set["source"] {
		opts = append(opts, go3up.WithSource(source))
	}

	if set["cachefile"] {
		opts = append(opts, go3up.WithCacheFile(cacheFile))
	}

	if set["region"] {
		opts = append(opts, go3up.WithRegion(region))
	}

	if set["profile"] {
		opts = append(opts, go3up.WithProfile(profile))
	}

	if set["encrypt"] {
		opts = append(opts, go3up.WithEncryption(encrypt))
	}

	if save {
		if err := saveConfig(cfgFile, set, go3up.Config{
			BucketName: bucket, Source: source, CacheFile: cacheFile,
			Region: region, Profile: profile, WorkersCount: workers, Encrypt: encrypt,
		}); err != nil {
			return fail(out, SetupFailed, "Failed to save config:", err)
		}
	}

	client, err := go3up.New(opts...)
	if err != nil {
		fail(out, CmdLineOptionError, fmt.Sprintf("Required field missing: %v.\n\nUsage:", err))
		fs.PrintDefaults()

		return CmdLineOptionError
	}

	if err = client.Run(context.Background()); err != nil {
		fail(out, SetupFailed, "Error:", err)

		switch {
		case errors.Is(err, go3up.ErrAuth):
			return S3AuthError
		case errors.Is(err, go3up.ErrCaching):
			return CachingFailure
		case errors.Is(err, go3up.ErrInvalidConfig):
			return CmdLineOptionError
		default:
			return SetupFailed
		}
	}

	return Success
}

// saveConfig writes the merged configuration (file values overlaid with
// the explicitly set flags) back to the config file.
func saveConfig(cfgFile string, set map[string]bool, flags go3up.Config) error {
	cfg, err := go3up.LoadConfig(cfgFile)
	if err != nil {
		return err
	}

	if set["bucket"] {
		cfg.BucketName = flags.BucketName
	}

	if set["source"] {
		cfg.Source = flags.Source
	}

	if set["cachefile"] {
		cfg.CacheFile = flags.CacheFile
	}

	if set["region"] {
		cfg.Region = flags.Region
	}

	if set["profile"] {
		cfg.Profile = flags.Profile
	}

	if set["workers"] {
		cfg.WorkersCount = flags.WorkersCount
	}

	if set["encrypt"] {
		cfg.Encrypt = flags.Encrypt
	}

	return cfg.Save(cfgFile)
}

// fail prints msg to out and returns the given exit code, or SetupFailed if printing failed.
func fail(out io.Writer, code int, msg ...any) int {
	if _, err := fmt.Fprintln(out, msg...); err != nil {
		return SetupFailed
	}

	return code
}
