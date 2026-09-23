// Package go3up (Go S3 Uploader) is a small S3 uploader tool.
//
// It was created in order to speed up S3 uploads by employing a local caching of files' md5 sums.
// That way, on subsequent runs, go3up can compute a list of the files that changed since the
// last upload and only upload those.
//
// The initial use case was a large static site (with 10k+ files) that frequently changed only
// a small subset of files (about ~100 routinely). In that particular case, the time reduction by
// switching from s3cmd to go3up was significant.
//
// On uploads with empty cache there may not be any benefit.
//
// The current focus of the tool is just one way/uploads (without deleting things that were removed
// locally, yet). That may (or not) change in the future.
package go3up

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// Logger receives pre-formatted messages. The default logger prints to stdout.
type Logger func(msg string)

// objectUploader uploads objects to S3. The real *transfermanager.Client satisfies it.
type objectUploader interface {
	UploadObject(ctx context.Context, input *transfermanager.UploadObjectInput, opts ...func(*transfermanager.Options)) (*transfermanager.UploadObjectOutput, error)
}

// stsAPI retrieves the AWS caller identity. The real *sts.Client satisfies it.
type stsAPI interface {
	GetCallerIdentity(ctx context.Context, input *sts.GetCallerIdentityInput, opts ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

// Client holds all the state needed to sync a local folder to an S3 bucket.
type Client struct {
	s3                objectUploader
	loadConfig        func(context.Context, ...func(*config.LoadOptions) error) (aws.Config, error)
	newUploader       func(aws.Config) objectUploader
	newSTS            func(aws.Config) stsAPI
	uploader          uploader
	logger            Logger
	bucketName        string
	source            string
	cacheFile         string
	region            string
	profile           string
	recoverableErrors []string
	headerRules       []HeaderRule
	retryBaseDelay    time.Duration
	workersCount      int
	dryRun            bool
	doUpload          bool
	doCache           bool
	quiet             bool
	verbose           bool
	encrypt           bool
}

// Option configures a Client.
type Option func(*Client)

// Sentinel errors, so that callers (e.g. the CLI) can map failures to exit codes.
var (
	ErrInvalidConfig = errors.New("invalid configuration")
	ErrAuth          = errors.New("s3 authentication failed")
	ErrCaching       = errors.New("caching failed")
)

var (
	_ objectUploader = (*transfermanager.Client)(nil)
	_ stsAPI         = (*sts.Client)(nil)
)

func WithBucket(name string) Option     { return func(c *Client) { c.bucketName = name } }
func WithSource(dir string) Option      { return func(c *Client) { c.source = dir } }
func WithCacheFile(path string) Option  { return func(c *Client) { c.cacheFile = path } }
func WithRegion(region string) Option   { return func(c *Client) { c.region = region } }
func WithProfile(profile string) Option { return func(c *Client) { c.profile = profile } }
func WithEncryption(on bool) Option     { return func(c *Client) { c.encrypt = on } }
func WithDryRun(on bool) Option         { return func(c *Client) { c.dryRun = on } }
func WithUpload(on bool) Option         { return func(c *Client) { c.doUpload = on } }
func WithCache(on bool) Option          { return func(c *Client) { c.doCache = on } }
func WithVerbose(on bool) Option        { return func(c *Client) { c.verbose = on } }
func WithQuiet(on bool) Option          { return func(c *Client) { c.quiet = on } }
func WithLogger(l Logger) Option        { return func(c *Client) { c.logger = l } }
func WithUploader(up uploader) Option   { return func(c *Client) { c.uploader = up } }
func WithRetryBaseDelay(d time.Duration) Option {
	return func(c *Client) { c.retryBaseDelay = d }
}

// WithWorkers sets the number of upload workers. Values < 1 are ignored.
func WithWorkers(n int) Option {
	return func(c *Client) {
		if n > 0 {
			c.workersCount = n
		}
	}
}

// WithHeaderRules replaces the default path to headers mappings.
func WithHeaderRules(rules ...HeaderRule) Option {
	return func(c *Client) { c.headerRules = rules }
}

// WithRecoverableErrors replaces the default list of recoverable S3 error suffixes.
func WithRecoverableErrors(suffixes ...string) Option {
	return func(c *Client) { c.recoverableErrors = suffixes }
}

// New creates a Client with sane defaults, applies the given options and validates the result.
func New(opts ...Option) (*Client, error) {
	c := &Client{
		workersCount:      runtime.NumCPU() * 2,
		source:            "output",
		cacheFile:         ".go3up.txt",
		region:            os.Getenv("AWS_REGION"),
		profile:           os.Getenv("AWS_PROFILE"),
		doUpload:          true,
		doCache:           true,
		retryBaseDelay:    100 * time.Millisecond, //nolint:mnd // ok
		headerRules:       defaultHeaderRules(),
		recoverableErrors: defaultRecoverableErrors(),
		logger:            defaultLogger,
		loadConfig:        config.LoadDefaultConfig,
		newUploader:       func(cfg aws.Config) objectUploader { return transfermanager.New(s3.NewFromConfig(cfg)) },
		newSTS:            func(cfg aws.Config) stsAPI { return sts.NewFromConfig(cfg) },
	}

	for _, opt := range opts {
		opt(c)
	}

	if err := c.validate(); err != nil {
		return nil, err
	}

	return c, nil
}

// validate checks that the mandatory settings are present and the paths exist.
func (c *Client) validate() (err error) {
	if c.bucketName == "" {
		return fmt.Errorf("%w: Bucket Name is not set", ErrInvalidConfig)
	}

	for label, val := range map[string]string{"Source": c.source, "Cache file": c.cacheFile} {
		if _, err = os.Stat(val); err != nil {
			return fmt.Errorf("%w: %s: %w", ErrInvalidConfig, label, err)
		}
	}

	return
}

// initS3 lazily initializes the AWS S3 client. It is only called when actually uploading,
// so dry runs and custom uploaders never touch AWS configuration.
func (c *Client) initS3(ctx context.Context) (err error) {
	cfg, err := c.loadConfig(ctx, config.WithRegion(c.region), config.WithSharedConfigProfile(c.profile))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAuth, err)
	}

	if c.verbose {
		var identity *sts.GetCallerIdentityOutput

		identity, err = c.newSTS(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
		if err != nil {
			return fmt.Errorf("%w: %w", ErrAuth, err)
		}

		c.say("AWS Identity: " + *identity.Arn)
	}

	c.s3 = c.newUploader(cfg)

	return
}
