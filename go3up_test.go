package main

import "testing"

func TestBetterMime(t *testing.T) {
	assertions := map[string]string{
		".html": "text/html; charset=utf-8",
		".jpg":  "image/jpeg",
		".JPG":  "image/jpeg",
		".ttf":  "binary/octet-stream",
	}

	for ext, mime := range assertions {
		if expected := betterMime(ext); expected != mime {
			t.Error("Expected", mime, "got", expected)
		}
	}
}

func init() {
	// set options for testing
}
