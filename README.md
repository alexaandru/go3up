# Go S3 Uploader

[![Test](https://github.com/alexaandru/go3up/actions/workflows/ci.yml/badge.svg)](https://github.com/alexaandru/go3up/actions/workflows/ci.yml)
![Coverage](coverage-badge.svg)
[![GoDoc](https://godoc.org/github.com/alexaandru/go3up?status.png)](https://godoc.org/github.com/alexaandru/go3up)

Go3Up (Go S3 Uploader) is a small S3 uploader, usable both as a CLI tool and as a Go library.

It was created in order to speed up S3 uploads by employing a local caching of files' md5 sums.
That way, on subsequent runs, go3up can compute a list of the files that changed since the
last upload and only upload those.

The initial use case was a large static site (with 10k+ files) that frequently changed only
a small subset of files (about ~100 routinely). In that particular case, the time reduction by
switching from s3cmd to go3up was significant.

On uploads with empty cache there may not be any benefit.

The current focus of the tool is just one way/uploads (without deleting things that were removed
locally, yet).

## CLI usage

Build with `make build` (or `go build -o go3up ./cmd/go3up`), then run `go3up -h` to get the help.
You can save your preferences to a .go3up.json config file by passing your command line flags as
usual and adding "-save" at the end.

For authentication, see http://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html
as we pretty much support all of those options, in this order: shared profile; EC2 role; env vars.

Exit codes: `0` success, `1` setup failed, `2` S3 auth error, `3` command line option error,
`4` caching failure.

## Library usage

```go
package main

import (
	"context"
	"fmt"

	"github.com/alexaandru/go3up"
)

func main() {
	client, err := go3up.New(
		go3up.WithBucket("my-bucket"),
		go3up.WithSource("output"),
		go3up.WithCacheFile(".go3up.txt"),
		go3up.WithRegion("us-east-1"),
		go3up.WithProfile("default"),
		go3up.WithWorkers(8),
		go3up.WithVerbose(true),
	)
	if err != nil {
		// invalid configuration (missing bucket, non-existent paths, ...)
		panic(err)
	}

	if err := client.Run(context.Background()); err != nil {
		// errors.Is(err, go3up.ErrAuth), errors.Is(err, go3up.ErrCaching), ...
		fmt.Println("upload failed:", err)
	}
}
```

Everything is configured through functional options — there is no global state. Notable options:

- `WithDryRun(true)` — compute the diff and log what would happen, without uploading or updating the cache.
- `WithUpload(false)` / `WithCache(false)` — skip the upload or the cache update phase.
- `WithLogger(fn)` — plug in your own logging (default prints to stdout).
- `WithUploader(fn)` — swap out the S3 upload implementation (useful for testing).
- `WithHeaderRules(rules...)` — custom path-pattern → headers mappings (Cache-Control, gzip, ...).
- `WithRecoverableErrors(suffixes...)` — customize which S3 errors are retried (with exponential backoff).
- `WithRetryBaseDelay(d)` — base delay for the retry backoff.

The JSON config file format is also available to library consumers via `go3up.LoadConfig`,
`Config.Save` and `Config.Options()` (which converts a loaded config into client options).

Errors are returned, never panics; the sentinel errors `go3up.ErrInvalidConfig`, `go3up.ErrAuth`
and `go3up.ErrCaching` allow callers to distinguish failure modes.

## TODO

- implement (optional) deletion of remote files missing on local.
