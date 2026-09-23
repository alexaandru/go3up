package go3up

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// mockSTS is a mock stsAPI returning a fixed identity or error.
type mockSTS struct {
	arn string
	err error
}

func TestNew(t *testing.T) {
	t.Run("validation", func(t *testing.T) {
		tests := []struct {
			name    string
			opts    []Option
			wantErr bool
		}{
			{"valid", []Option{WithBucket("example_bucket"), WithSource("testdata/output"), WithCacheFile("testdata/.go3up.txt")}, false},
			{"missing bucket", []Option{WithSource("testdata/output"), WithCacheFile("testdata/.go3up.txt")}, true},
			{"bogus source", []Option{WithBucket("example_bucket"), WithSource("testdata/bogus"), WithCacheFile("testdata/.go3up.txt")}, true},
			{"bogus cache file", []Option{WithBucket("example_bucket"), WithSource("testdata/output"), WithCacheFile("testdata/bogus.txt")}, true},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				_, err := New(tc.opts...)

				if tc.wantErr && !errors.Is(err, ErrInvalidConfig) {
					t.Errorf("Expected ErrInvalidConfig, got %v", err)
				}

				if !tc.wantErr && err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			})
		}
	})

	t.Run("options applied", func(t *testing.T) {
		up := func(*sourceFile) error { return nil }
		rules := []HeaderRule{{PathPattern: regexp.MustCompile("foo"), Headers: map[string]string{}}}
		logger := func(string) {}

		c := newTestClient(t,
			WithRegion("us-west-1"),
			WithProfile("test-profile"),
			WithWorkers(42),
			WithEncryption(true),
			WithDryRun(true),
			WithUpload(false),
			WithCache(false),
			WithVerbose(true),
			WithUploader(up),
			WithHeaderRules(rules...),
			WithRecoverableErrors("custom error"),
			WithLogger(logger),
		)

		if c.region != "us-west-1" || c.profile != "test-profile" || c.workersCount != 42 {
			t.Errorf("Expected region/profile/workers to be applied, got %s/%s/%d", c.region, c.profile, c.workersCount)
		}

		if !c.encrypt || !c.dryRun || c.doUpload || c.doCache || !c.verbose {
			t.Errorf("Expected bool flags to be applied, got %v", c)
		}

		if c.uploader == nil || c.logger == nil {
			t.Error("Expected uploader and logger to be applied")
		}

		if len(c.headerRules) != 1 || len(c.recoverableErrors) != 1 {
			t.Error("Expected header rules and recoverable errors to be replaced")
		}
	})

	t.Run("invalid workers ignored", func(t *testing.T) {
		c := newTestClient(t, WithWorkers(0))
		if c.workersCount <= 0 {
			t.Error("Expected WithWorkers(0) to be ignored, got", c.workersCount)
		}
	})
}

func TestInitS3(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		buf := &bytes.Buffer{}
		c := newTestClient(t, WithVerbose(true), WithLogger(newTestLogger(buf)))
		c.loadConfig = fakeLoadConfig
		c.newSTS = func(aws.Config) stsAPI { return mockSTS{arn: "arn:aws:iam::1234:user/test"} }
		c.newUploader = func(aws.Config) objectUploader { return &mockUploader{} }

		if err := c.initS3(t.Context()); err != nil {
			t.Fatal("Expected initS3 to succeed, got", err)
		}

		if c.s3 == nil {
			t.Error("Expected the uploader to be initialized")
		}

		if !strings.Contains(buf.String(), "arn:aws:iam::1234:user/test") {
			t.Error("Expected the identity to be logged, got", buf.String())
		}
	})

	t.Run("config load error", func(t *testing.T) {
		c := newTestClient(t)
		c.loadConfig = func(context.Context, ...func(*config.LoadOptions) error) (aws.Config, error) {
			return aws.Config{}, errors.New("boom")
		}

		if err := c.initS3(t.Context()); !errors.Is(err, ErrAuth) {
			t.Error("Expected ErrAuth, got", err)
		}
	})

	t.Run("sts error", func(t *testing.T) {
		c := newTestClient(t, WithVerbose(true))
		c.loadConfig = fakeLoadConfig
		c.newSTS = func(aws.Config) stsAPI { return mockSTS{err: errors.New("boom")} }
		c.newUploader = func(aws.Config) objectUploader { return &mockUploader{} }

		if err := c.initS3(t.Context()); !errors.Is(err, ErrAuth) {
			t.Error("Expected ErrAuth, got", err)
		}
	})
}

func (m mockSTS) GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	if m.err != nil {
		return nil, m.err
	}

	return &sts.GetCallerIdentityOutput{Arn: &m.arn}, nil
}

// testOptions returns the base options shared by all tests, with extras appended.
func testOptions(extra ...Option) []Option {
	base := make([]Option, 0, 6+len(extra))
	base = append(base,
		WithBucket("example_bucket"),
		WithSource("testdata/output"),
		WithCacheFile("testdata/.go3up.txt"),
		WithQuiet(true),
		WithLogger(newTestLogger(&bytes.Buffer{})),
		WithRetryBaseDelay(time.Nanosecond),
	)

	return append(base, extra...)
}

// newTestClient creates a Client with the test defaults plus any extra options.
func newTestClient(t *testing.T, extra ...Option) *Client {
	t.Helper()

	c, err := New(testOptions(extra...)...)
	if err != nil {
		t.Fatal("Failed to create test client:", err)
	}

	return c
}

// newTestLogger returns a concurrency-safe Logger writing to buf.
func newTestLogger(buf *bytes.Buffer) Logger {
	mu := &sync.Mutex{}

	return func(msg string) {
		mu.Lock()
		defer mu.Unlock()

		_, _ = buf.WriteString(msg)
	}
}
