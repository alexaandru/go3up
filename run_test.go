package go3up

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/alexaandru/utils"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
)

// mockUploader is a mock objectUploader recording its inputs and bodies.
type mockUploader struct {
	inputs []*transfermanager.UploadObjectInput
	bodies [][]byte
	err    error
	mu     sync.Mutex
}

const (
	_ = iota
	noError
	recoverableError
	fatalError
)

func TestRun(t *testing.T) {
	setup := func(t *testing.T) (source, cache string) {
		t.Helper()

		dir := t.TempDir()
		source = filepath.Join(dir, "output")
		cache = filepath.Join(dir, ".go3up.txt")

		if err := os.Mkdir(source, 0o755); err != nil {
			t.Fatal(err)
		}

		for name, content := range map[string]string{"barbaz.txt": "barbaz", "foobar.html": "foobar"} {
			if err := os.WriteFile(filepath.Join(source, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		if _, err := os.Create(cache); err != nil {
			t.Fatal(err)
		}

		return
	}

	newClient := func(t *testing.T, source, cache string, extra ...Option) (*Client, *[]*sourceFile) {
		t.Helper()

		upFn, uploads := fakeUploaderGen()
		opts := append([]Option{WithSource(source), WithCacheFile(cache), WithUploader(upFn)}, extra...)

		return newTestClient(t, opts...), uploads
	}

	cacheContent := func(t *testing.T, cache string) string {
		t.Helper()

		b, err := os.ReadFile(cache)
		if err != nil {
			t.Fatal(err)
		}

		return string(b)
	}

	t.Run("uploads changed files and updates cache", func(t *testing.T) {
		source, cache := setup(t)
		c, uploads := newClient(t, source, cache)

		if err := c.Run(t.Context()); err != nil {
			t.Fatal("Run failed:", err)
		}

		if len(*uploads) != 2 {
			t.Fatal("Expected 2 uploads, got", len(*uploads))
		}

		if content := cacheContent(t, cache); !strings.Contains(content, "foobar.html") {
			t.Error("Expected cache to be updated, got", content)
		}
	})

	t.Run("nothing to upload", func(t *testing.T) {
		source, cache := setup(t)
		c, uploads := newClient(t, source, cache)

		if err := utils.FileHashesNew(source).Dump(cache); err != nil {
			t.Fatal("Failed to prefill cache:", err)
		}

		if err := c.Run(t.Context()); err != nil {
			t.Fatal("Run failed:", err)
		}

		if len(*uploads) != 0 {
			t.Error("Expected no uploads, got", len(*uploads))
		}
	})

	t.Run("dry run", func(t *testing.T) {
		source, cache := setup(t)
		c, uploads := newClient(t, source, cache, WithDryRun(true))

		if err := c.Run(t.Context()); err != nil {
			t.Fatal("Run failed:", err)
		}

		if len(*uploads) != 0 {
			t.Error("Expected no uploads on dry run, got", len(*uploads))
		}

		if content := cacheContent(t, cache); content != "" {
			t.Error("Expected cache to remain empty on dry run, got", content)
		}
	})

	t.Run("upload skipped", func(t *testing.T) {
		source, cache := setup(t)
		c, uploads := newClient(t, source, cache, WithUpload(false))

		if err := c.Run(t.Context()); err != nil {
			t.Fatal("Run failed:", err)
		}

		if len(*uploads) != 0 {
			t.Error("Expected no uploads, got", len(*uploads))
		}

		if content := cacheContent(t, cache); !strings.Contains(content, "foobar.html") {
			t.Error("Expected cache to be updated, got", content)
		}
	})

	t.Run("cache skipped", func(t *testing.T) {
		source, cache := setup(t)
		c, _ := newClient(t, source, cache, WithCache(false))

		if err := c.Run(t.Context()); err != nil {
			t.Fatal("Run failed:", err)
		}

		if content := cacheContent(t, cache); content != "" {
			t.Error("Expected cache to remain empty, got", content)
		}
	})

	t.Run("caching failure", func(t *testing.T) {
		source, cache := setup(t)

		if err := os.Chmod(cache, 0o444); err != nil { // Dump's os.Create will fail on it.
			t.Fatal(err)
		}

		c, _ := newClient(t, source, cache)

		if err := c.Run(t.Context()); !errors.Is(err, ErrCaching) {
			t.Error("Expected ErrCaching, got", err)
		}
	})
}

func TestFilesLists(t *testing.T) {
	c := newTestClient(t, WithCacheFile("testdata/.cacheEmpty.txt"))
	current, diff := c.filesLists()

	if current["barbaz.txt"] != "dac2e8bd758efb58a30f9fcd7ac28b1b" ||
		current["foobar.html"] != "01677e4c0ae5468b9b8b823487f14524" {
		t.Error("Current list does not match expectation")
	}

	sort.Strings(diff)

	if strings.Join(diff, ":") != "barbaz.txt:foobar.html" {
		t.Error("Expected diff to hold barbaz.txt and foobar.html")
	}
}

func TestUpload(t *testing.T) {
	runUploads := func(t *testing.T, c *Client, upFn uploader, rejected *syncedlist, workers int) {
		t.Helper()

		up := make(chan *sourceFile)
		wgUploads, wgWorkers := new(sync.WaitGroup), new(sync.WaitGroup)

		wgUploads.Add(2)
		wgWorkers.Add(workers)

		for range workers {
			go c.upload(upFn, up, rejected, wgUploads, wgWorkers)
		}

		up <- c.newSourceFile("foobar.html")

		up <- c.newSourceFile("barbaz.txt")

		wgUploads.Wait()
		close(up)
		wgWorkers.Wait()
	}

	t.Run("success", func(t *testing.T) {
		upFn, uploads := fakeUploaderGen()
		c := newTestClient(t, WithUploader(upFn))
		rejected := &syncedlist{}

		runUploads(t, c, upFn, rejected, 1)

		if len(*uploads) != 2 {
			t.Fatal("Expected to upload 2 files, got", *uploads)
		}
	})

	t.Run("dry run", func(t *testing.T) {
		upFn, uploads := fakeUploaderGen()
		c := newTestClient(t, WithUploader(upFn), WithDryRun(true))
		rejected := &syncedlist{}

		runUploads(t, c, upFn, rejected, 1)

		if len(*uploads) > 0 {
			t.Fatal("Expected to get a blank uploads list, got", *uploads)
		}
	})

	t.Run("unrecoverable", func(t *testing.T) {
		upFn, uploads := fakeUploaderGen(fatalError)
		c := newTestClient(t, WithUploader(upFn))
		rejected := &syncedlist{}

		runUploads(t, c, upFn, rejected, 1)

		if len(*uploads) != 2 {
			t.Fatal("Expected both uploads to be processed, got", *uploads)
		}

		if len(rejected.list) != 2 {
			t.Fatal("Expected all of the uploads to be rejected, got", rejected.list)
		}
	})

	t.Run("recoverable", func(t *testing.T) {
		upFn, uploads := fakeUploaderGen(recoverableError)
		c := newTestClient(t, WithUploader(upFn))
		rejected := &syncedlist{}

		runUploads(t, c, upFn, rejected, 2)

		if lu := len(*uploads); lu != 2*maxTries {
			t.Fatal("Expected both uploads to be processed maxTries, got", lu, "attempts")
		}

		if len(rejected.list) != 2 {
			t.Fatal("Expected all of the uploads to be rejected, got", rejected.list)
		}
	})
}

func TestS3putGen(t *testing.T) {
	dir := t.TempDir()

	for name, content := range map[string]string{"barbaz.txt": "not compressed", "foobar.html": "compressed"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mock := &mockUploader{}
	c := newTestClient(t, WithSource(dir), WithEncryption(true))
	c.loadConfig = fakeLoadConfig
	c.newUploader = func(aws.Config) objectUploader { return mock }

	upFn, err := c.s3putGen(t.Context())
	if err != nil {
		t.Fatal("Expected s3putGen to succeed, got", err)
	}

	for _, fname := range []string{"barbaz.txt", "foobar.html"} {
		if err := upFn(c.newSourceFile(fname)); err != nil {
			t.Fatal("Upload failed:", err)
		}
	}

	if len(mock.inputs) != 2 {
		t.Fatal("Expected 2 uploads, got", len(mock.inputs))
	}

	plain, zipped := mock.inputs[0], mock.inputs[1]

	if *plain.Bucket != "example_bucket" || *plain.Key != "barbaz.txt" {
		t.Errorf("Unexpected bucket/key: %s/%s", *plain.Bucket, *plain.Key)
	}

	if plain.ContentEncoding != nil {
		t.Error("Expected no content encoding for .txt, got", *plain.ContentEncoding)
	}

	if string(mock.bodies[0]) != "not compressed" {
		t.Error("Expected plain body, got", string(mock.bodies[0]))
	}

	if *zipped.ContentEncoding != "gzip" {
		t.Error("Expected gzip encoding for .html, got", *zipped.ContentEncoding)
	}

	zr, err := gzip.NewReader(bytes.NewReader(mock.bodies[1]))
	if err != nil {
		t.Fatal("Expected a gzipped body:", err)
	}

	body, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal("Failed to decompress body:", err)
	}

	if string(body) != "compressed" {
		t.Error("Expected decompressed body 'compressed', got", string(body))
	}

	if plain.CacheControl != nil {
		t.Error("Expected no cache control for .txt, got", *plain.CacheControl)
	}

	if *zipped.CacheControl != "max-age=3600" {
		t.Error("Expected cache control max-age=3600, got", *zipped.CacheControl)
	}
}

// fakeLoadConfig is a fake AWS config loader; it never touches the network or the environment.
func fakeLoadConfig(context.Context, ...func(*config.LoadOptions) error) (aws.Config, error) {
	return aws.Config{Region: "us-east-1"}, nil
}

func (m *mockUploader) UploadObject(_ context.Context, input *transfermanager.UploadObjectInput, _ ...func(*transfermanager.Options)) (*transfermanager.UploadObjectOutput, error) {
	body, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.inputs = append(m.inputs, input)
	m.bodies = append(m.bodies, body)

	if m.err != nil {
		return nil, m.err
	}

	return &transfermanager.UploadObjectOutput{}, nil
}

// fakeUploaderGen returns a fake uploader that records the files it was called with,
// optionally failing with recoverable or fatal errors.
func fakeUploaderGen(opts ...int) (fn uploader, out *[]*sourceFile) {
	errorKind, m := noError, sync.Mutex{}
	if len(opts) > 0 {
		errorKind = opts[0]
	}

	out = &[]*sourceFile{}
	fn = func(src *sourceFile) (err error) {
		m.Lock()
		defer m.Unlock()

		*out = append(*out, src)

		switch errorKind {
		case noError:
			return
		case recoverableError:
			return errors.New("Something something. " + defaultRecoverableErrors()[0])
		}

		return errors.New("Some made up error")
	}

	return
}
