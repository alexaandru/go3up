package main

import (
	"bytes"
	"errors"
	"sync"
	"testing"
)

const (
	_ = iota
	noError
	recoverableError
	fatalError
)

var fakeBuffer *bytes.Buffer

func TestValidateCmdLineFlags(t *testing.T) {
	opts1 := &options{BucketName: "example_bucket", Source: "test/output", CacheFile: "test/.go3up.txt", Region: "us-west-1"}
	if err := validateCmdLineFlags(opts1); err != nil {
		t.Errorf("Expected %v to pass validation", opts1)
	}

	opts1 = &options{BucketName: "", Source: "test/output", CacheFile: "test/.go3up.txt"}
	if err := validateCmdLineFlags(opts1); err == nil {
		t.Error("Expected to fail validation")
	}
}

func TestValidateCmdLineFlag(t *testing.T) {
	if err := validateCmdLineFlag("output folder", "test/output"); err != nil {
		t.Error("Expected test/output to pass validation")
	}

	if err := validateCmdLineFlag("output folder", "test/bogus"); err == nil {
		t.Error("Expected test/bogus to fail validation")
	}

	if err := validateCmdLineFlag("Bucket Name", "foobar"); err != nil {
		t.Error("Expected foobar bucket name to pass validation")
	}

	if err := validateCmdLineFlag("Bucket Name", ""); err == nil {
		t.Error("Expected foobar bucket name to fail validation")
	}
}

func fakeUploaderGen(opts ...int) (fn uploader, out *([]*sourceFile)) {
	errorKind, m := noError, sync.Mutex{}
	if len(opts) > 0 {
		errorKind = opts[0]
	}

	out = &[]*sourceFile{}
	fn = func(src *sourceFile) (err error) {
		m.Lock()
		*out = append(*out, src)
		m.Unlock()

		if errorKind == noError {
			return
		} else if errorKind == recoverableError {
			return errors.New("Something something. " + recoverableErrorsSuffixes[0])
		}

		return errors.New("Some made up error")
	}

	return
}

var _ = func() bool {
	testing.Init()
	return true
}()

func init() {
	opts.BucketName = "example_bucket"
	opts.Source = "test/output"
	opts.CacheFile = "test/.go3up.txt"
	appEnv = "test"
	fakeBuffer := &bytes.Buffer{}
	sayLock := &sync.Mutex{}
	sayFn := loggerGen(fakeBuffer)
	say = func(msg ...string) {
		sayLock.Lock()
		defer sayLock.Unlock()
		sayFn(msg...)
	}
}
