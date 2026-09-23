package go3up

import (
	"sync"
	"testing"
)

func TestHeadersMerge(t *testing.T) {
	h1, h2 := headers{"foo": "foo1", "bar": "bar1"},
		headers{"baz": "baz1"}
	expected := headers{"foo": "foo1", "bar": "bar1", "baz": "baz1"}

	h1.merge(h2)

	if !h1.equal(expected) {
		t.Errorf("Expected %v to equal %v", h1, expected)
	}
}

func TestHeadersEqual(t *testing.T) {
	h1, h2, h3 := headers{"foo": "foo1", "bar": "bar1"},
		headers{"foo": "foo1", "bar": "bar1"},
		headers{"foo": "foo1"}

	if !h1.equal(h2) {
		t.Errorf("Expected %v to equal %v", h1, h2)
	}

	if h1.equal(h3) {
		t.Errorf("Expected %v NOT to equal %v", h1, h3)
	}
}

func TestNewSourceFile(t *testing.T) {
	c := newTestClient(t)
	fname := "foobar.html"
	sf := c.newSourceFile(fname)
	expectedHdrs := headers{ContentType: "text/html; charset=utf-8", ContentEncoding: "gzip", CacheControl: "max-age=3600"}

	if sf.fname != fname {
		t.Errorf("Expected fname to be set to %s got %s", fname, sf.fname)
	}

	if fpath := c.source + "/" + fname; sf.fpath != fpath {
		t.Errorf("Expected fpath to be set to %s got %s", fpath, sf.fpath)
	}

	if !sf.hdrs.equal(expectedHdrs) {
		t.Errorf("Expected hdrs to be set to %v got %v", expectedHdrs, sf.hdrs)
	}

	if !sf.gzip {
		t.Error("Expected .html files to be compressed")
	}

	tests := map[string]string{
		"articole/foobar.html": "3600",
		"articole/index.html":  "1800",
		"index.html":           "1800",
	}

	for fname, ttl := range tests {
		sf = c.newSourceFile(fname)

		expectedHdrs = headers{ContentType: "text/html; charset=utf-8", ContentEncoding: "gzip", CacheControl: "max-age=" + ttl}
		if !sf.hdrs.equal(expectedHdrs) {
			t.Errorf("Expected hdrs to be set to %v got %v", expectedHdrs, sf.hdrs)
		}
	}
}

func TestGetHeader(t *testing.T) {
	tests := []struct {
		name    string
		encrypt bool
		hdr     string
		want    string
	}{
		{"encryption on", true, Encryption, sse},
		{"encryption off", false, Encryption, ""},
		{"existing header", false, ContentType, "text/html; charset=utf-8"},
		{"missing header", false, "X-Bogus", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, WithEncryption(tc.encrypt))
			sf := c.newSourceFile("foobar.html")
			got := sf.getHeader(tc.hdr)

			if tc.want == "" {
				if got != nil {
					t.Errorf("Expected nil, got %v", *got)
				}

				return
			}

			if got == nil || *got != tc.want {
				t.Errorf("Expected %q, got %v", tc.want, got)
			}
		})
	}
}

func TestRecordAttempt(t *testing.T) {
	c := newTestClient(t)
	fname := "foobar.html"
	sf := c.newSourceFile(fname)
	wg := new(sync.WaitGroup)

	wg.Add(2)

	go func() {
		for range 1000 {
			sf.recordAttempt()
		}

		wg.Done()
	}()

	go func() {
		for range 1000 {
			sf.recordAttempt()
		}

		wg.Done()
	}()

	wg.Wait()

	if sf.attempts != 2000 {
		t.Error("Expected 2000 attempts, got", sf.attempts)
	}
}

func TestRetriable(t *testing.T) {
	c := newTestClient(t)
	fname := "foobar.html"
	sf := c.newSourceFile(fname)

	if sf.attempts = 0; !sf.retriable() {
		t.Fatal("A source file with no attempts should be retriable")
	}

	if sf.attempts = maxTries - 1; !sf.retriable() {
		t.Fatal("A source file with less attempts than maxTries should be retriable")
	}

	if sf.attempts = maxTries; sf.retriable() {
		t.Fatal("A source file with exactly maxTries attempts should NOT be retriable")
	}

	if sf.attempts = maxTries + 1; sf.retriable() {
		t.Fatal("A source file with more than maxTries attempts should NOT be retriable")
	}
}
