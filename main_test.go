package main

import (
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
	t.Skip() // this one should test main() function and ensure program works from one end to another
}
