package go3up

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"math"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/alexaandru/utils"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager/types"
)

type uploader func(*sourceFile) error

// max number of attempts to retry a failed upload.
const maxTries = 10

// Run uploads all changed files from the source folder to the S3 bucket and updates the cache.
func (c *Client) Run(ctx context.Context) error {
	up := c.uploader
	if up == nil {
		var err error

		up, err = c.s3putGen(ctx)
		if err != nil {
			return err
		}
	}

	current, diff := c.filesLists()
	if len(diff) == 0 {
		c.say("Nothing to upload.", "Nothing to upload.\n")
		return nil
	}

	c.say(fmt.Sprintf("There are %d files to be uploaded to '%s'", len(diff), c.bucketName), "Uploading ")

	rejected := &syncedlist{}

	if c.doUpload {
		uploads := make(chan *sourceFile)
		wgUploads, wgWorkers := new(sync.WaitGroup), new(sync.WaitGroup)

		wgUploads.Add(len(diff))
		wgWorkers.Add(c.workersCount)

		for range c.workersCount {
			go c.upload(up, uploads, rejected, wgUploads, wgWorkers)
		}

		sort.Strings(diff)

		for _, fname := range diff {
			uploads <- c.newSourceFile(fname)
		}

		wgUploads.Wait()
		close(uploads)
		wgWorkers.Wait()
		c.say("Done uploading files.")
	} else {
		c.say("Skipping upload")
	}

	switch {
	case !c.doCache:
		c.say("Skipping cache.")
	case c.dryRun:
		c.say("Pretending to update cache.")
	default:
		current = current.Reject(rejected.list)
		if err := current.Dump(c.cacheFile); err != nil {
			return fmt.Errorf("%w: %w", ErrCaching, err)
		}

		c.say("Done updating cache.")
	}

	c.say("All done!", " done!\n")

	return nil
}

// filesLists returns both the current files list as well as the difference from the old (cached) files list.
func (c *Client) filesLists() (current utils.FileHashes, diff []string) {
	current = utils.FileHashesNew(c.source)
	old := utils.FileHashes{}
	old.Load(c.cacheFile)
	diff = current.Diff(old)

	return
}

// upload fetches sourceFiles from uploads chan, attempts to upload them and enqueue the results to
// completed list. On failure it attempts to retry, up to maxTries per source file.
func (c *Client) upload(fn uploader, uploads chan *sourceFile, rejected *syncedlist, wgUploads, wgWorkers *sync.WaitGroup) {
	defer wgWorkers.Done()

	for src := range uploads {
		if c.dryRun {
			c.say("Pretending to upload "+src.fname, ".")
			wgUploads.Done()

			continue
		}

		err := fn(src)
		if err == nil {
			wgUploads.Done()
			c.say("Uploaded "+src.fname, ".")

			continue
		}

		src.recordAttempt()

		if !src.retriable() || !c.isRecoverable(err) {
			rejected.add(src.fname)
			c.say("Failed to upload "+src.fname+": "+err.Error(), "F")
			wgUploads.Done()

			continue
		}

		go func() {
			c.say("Retrying "+src.fname, "r")

			wait := time.Duration(float64(c.retryBaseDelay) * math.Pow(2, float64(src.attempts)))
			<-time.After(wait)

			uploads <- src
		}()
	}
}

// s3putGen initializes the AWS client and generates an S3 upload func.
func (c *Client) s3putGen(ctx context.Context) (uploader, error) {
	if err := c.initS3(ctx); err != nil {
		return nil, err
	}

	return func(src *sourceFile) (err error) {
		f, err := os.Open(filepath.Join(c.source, src.fname))
		if err != nil {
			return err
		}

		defer func() {
			if cerr := f.Close(); cerr != nil && err == nil {
				err = cerr
			}
		}()

		var reader io.Reader = f

		cacheControl, contentEnc, contentType := src.getHeader(CacheControl), src.getHeader(ContentEncoding),
			mime.TypeByExtension(strings.ToLower(filepath.Ext(src.fname)))
		if src.gzip {
			rr, w := io.Pipe()
			wz := gzip.NewWriter(w)

			go func() {
				if _, cerr := io.Copy(wz, f); cerr != nil {
					_ = w.CloseWithError(fmt.Errorf("compression error: %w", cerr))
					return
				}

				if cerr := wz.Close(); cerr != nil {
					_ = w.CloseWithError(fmt.Errorf("compression error: %w", cerr))
					return
				}

				if cerr := w.Close(); cerr != nil {
					c.say("Failed to close gzip pipe: "+cerr.Error(), "F")
				}
			}()

			reader = rr
		}

		_, err = c.s3.UploadObject(ctx, &transfermanager.UploadObjectInput{
			Key:                  &src.fname,
			Body:                 reader,
			Bucket:               &c.bucketName,
			ContentType:          &contentType,
			ContentEncoding:      contentEnc,
			CacheControl:         cacheControl,
			ServerSideEncryption: types.ServerSideEncryptionAes256,
		})

		return err
	}, nil
}
