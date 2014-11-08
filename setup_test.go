package main

import "testing"

func TestValidateCmdLineFlags(t *testing.T) {
	opts1 := &options{bucketName: "example_bucket", source: "test/output", cacheFile: "test/.go3up.txt"}
	if err := validateCmdLineFlags(opts1); err != nil {
		t.Errorf("Expected %v to pass validation", opts1)
	}

	_ = &options{bucketName: "", source: "test/output", cacheFile: "test/.go3up.txt"}
	t.Skip("os.Exit again, skipping it...")
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

func init() {
	opts.bucketName = "example_bucket"
	opts.source = "test/output"
	opts.cacheFile = "test/.go3up.txt"
}
