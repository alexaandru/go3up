package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
)

const (
	_ = iota
	noError
	recoverableError
	fatalError
)

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

func TestCompletedListAdd(t *testing.T) {
	n, cl := 10, &completedList{}
	expectedArr := make([]string, n)
	wg := new(sync.WaitGroup)
	wg.Add(n)
	for i := 0; i < n; i++ {
		expectedArr[i] = fmt.Sprintf("%d", i)
		go func(i int) {
			defer wg.Done()
			cl.add(fmt.Sprintf("%d", i))
		}(i)
	}
	wg.Wait()
	expected := strings.Join(expectedArr, ":")
	sort.Strings(cl.list)
	actual := strings.Join(cl.list, ":")

	if expected != actual {
		t.Errorf("Expected %s\n got %s\n", expected, actual)
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

func init() {
	opts.bucketName = "example_bucket"
	opts.source = "test/output"
	opts.cacheFile = "test/.go3up.txt"
	appEnv = "test"
}
