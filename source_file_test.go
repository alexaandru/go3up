package main

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestHeadersMerge(t *testing.T) {
	h1, h2 := headers{"foo": {"foo1"}, "bar": {"bar1", "bar2"}},
		headersDef{"baz": "baz1"}
	expected := headers{"foo": {"foo1"}, "bar": {"bar1", "bar2"}, "baz": {"baz1"}}

	if h1.merge(h2); !h1.equal(expected) {
		t.Errorf("Expected %v to equal %v", h1, expected)
	}
}

func TestHeaderEqual(t *testing.T) {
	h1, h2, h3, h4 := headers{"foo": {"foo1"}, "bar": {"bar1", "bar2"}},
		headers{"foo": {"foo1"}, "bar": {"bar1", "bar2"}},
		headers{"foo": {"foo1"}, "bar": {"bar1"}},
		headers{"foo": {"foo1"}}

	if !h1.equal(h2) {
		t.Error("Expected %v to equal %v", h1, h2)
	}

	if h1.equal(h3) {
		t.Error("Expected %v NOT to equal %v", h1, h3)
	}

	if h1.equal(h4) {
		t.Error("Expected %v NOT to equal %v", h1, h4)
	}
}

func TestNewSourceFile(t *testing.T) {
	fname := "foobar.html"
	sf := newSourceFile(fname)
	expectedHdrs := headers{ContentType: {"text/html; charset=utf-8"}, ContentEncoding: {"gzip"}, CacheControl: {"max-age=86400"}}

	if sf.fname != fname {
		t.Errorf("Expected fname to be set to %s got %s", fname, sf.fname)
	}

	if fpath := opts.source + "/" + fname; sf.fpath != fpath {
		t.Errorf("Expected fpath to be set to %s got %s", fpath, sf.fpath)
	}

	if !sf.hdrs.equal(expectedHdrs) {
		t.Errorf("Expected hdrs to be set to %v got %v", expectedHdrs, sf.hdrs)
	}

	if !sf.gzip {
		t.Error("Expected .html files to be compressed")
	}
}

func TestSourceFileBody(t *testing.T) {
	fname := "foobar.html"
	sf := newSourceFile(fname)
	expectedContent := "1f8b080000096e8800ff72cbcf57704a2ce202040000ffff4f79994a08000000"
	expected, _ := hex.DecodeString(expectedContent)

	if actual := sf.body(); !bytes.Equal(expected, actual) {
		t.Error("Expected to get compressed content")
	}

	fname = "barbaz.txt"
	sf = newSourceFile(fname)
	expectedStr := "Bar Baz\n"

	if actual := sf.body(); string(actual) != expectedStr {
		t.Errorf("Expected to get %s got %s", expectedStr, actual)
	}
}
