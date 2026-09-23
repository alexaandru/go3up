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

	cfg, err := go3up.LoadConfig(cfgFile)
	if err != nil {
		return fail(out, SetupFailed, "Failed to load config:", err)
	}

	var dry, verbose, quiet, doUpload, doCache, save bool

	fs := flag.NewFlagSet("go3up", flag.ContinueOnError)
	fs.SetOutput(out)

	fs.IntVar(&cfg.WorkersCount, "workers", cfg.WorkersCount, "No. of workers to use for uploads")
	fs.StringVar(&cfg.BucketName, "bucket", cfg.BucketName, "Bucket to upload files to")
	fs.StringVar(&cfg.Source, "source", cfg.Source, "Source folder for files to be uploaded")
	fs.StringVar(&cfg.CacheFile, "cachefile", cfg.CacheFile, "Location of the cache file")
	fs.StringVar(&cfg.Region, "region", cfg.Region, "AWS region")
	fs.StringVar(&cfg.Profile, "profile", cfg.Profile, "AWS shared profile")
	fs.StringVar(&cfgFile, "cfgfile", cfgFile, "Config file location")
	fs.BoolVar(&dry, "dry", false, "Dry run (do not upload/update cache)")
	fs.BoolVar(&verbose, "verbose", false, "Print the name of the files as they are uploaded")
	fs.BoolVar(&quiet, "quiet", false, "Print only warnings and/or errors")
	fs.BoolVar(&doUpload, "upload", true, "Do perform an upload")
	fs.BoolVar(&doCache, "cache", true, "Do update the cache")
	fs.BoolVar(&cfg.Encrypt, "encrypt", cfg.Encrypt, "Encrypt files on server side")
	fs.BoolVar(&save, "save", false, "Saves the current commandline options to a config file")

	if err = fs.Parse(args); err != nil {
		return CmdLineOptionError
	}

	if cfgFile != defaultCfgFile { // We were given a different config file, use that instead.
		if cfg, err = go3up.LoadConfig(cfgFile); err != nil {
			return fail(out, SetupFailed, "Failed to load config:", err)
		}
	}

	if save {
		if err = cfg.Save(cfgFile); err != nil {
			return fail(out, SetupFailed, "Failed to save config:", err)
		}
	}

	opts := append(cfg.Options(),
		go3up.WithDryRun(dry),
		go3up.WithVerbose(verbose),
		go3up.WithQuiet(quiet),
		go3up.WithUpload(doUpload),
		go3up.WithCache(doCache),
	)

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

// fail prints msg to out and returns the given exit code, or SetupFailed if printing failed.
func fail(out io.Writer, code int, msg ...any) int {
	if _, err := fmt.Fprintln(out, msg...); err != nil {
		return SetupFailed
	}

	return code
}
