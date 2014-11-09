package main

import (
	"os"
	"sort"
	"strings"
	"testing"
)

func TestFilesList(t *testing.T) {
	cacheFile := opts.cacheFile
	opts.cacheFile = "test/.cacheEmpty.txt"
	current, diff := filesLists()

	if current["barbaz.txt"] != "dac2e8bd758efb58a30f9fcd7ac28b1b" ||
		current["foobar.html"] != "01677e4c0ae5468b9b8b823487f14524" {
		t.Error("Current list does not match expectation")
	}

	sort.Strings(diff)
	if strings.Join(diff, ":") != "barbaz.txt:foobar.html" {
		t.Error("Expected diff to hold barbaz.txt and foobar.html")
	}

	opts.cacheFile = cacheFile
}

func TestS3Put(t *testing.T) {
	t.Skip()
}

func TestIntegration(t *testing.T) {
	if _, err := os.Create(opts.cacheFile); err != nil {
		t.Fatal("Failed to truncate the cache file")
	}

	upFn, uploads := fakeUploaderGen()
	opts.upload = upFn

	main()

	fnames := make([]string, len(*uploads))
	for k, v := range *uploads {
		fnames[k] = v.fname
	}
	sort.Strings(fnames)
	if expected, actual := "barbaz.txt:foobar.html", strings.Join(fnames, ":"); expected != actual {
		t.Fatalf("Expected %s to be uploaded got %s", expected, actual)
	}
}
