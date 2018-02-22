package main

import (
	"mime"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Headers
const (
	ContentEncoding = "Content-Encoding"
	CacheControl    = "Cache-Control"
	ContentType     = "Content-Type"
	// pseudo headers
	Encryption = "EncryptionON"
)

var sse = "AES256"

type headers map[string]string

type pathToHeaders struct {
	pathPattern *regexp.Regexp
	headers
}

type sourceFile struct {
	fname,
	fpath string
	hdrs     headers
	gzip     bool
	attempts int
	sync.Mutex
}

func (h *headers) merge(other headers) {
	for key, val := range other {
		(*h)[key] = val
	}
}

func (h *headers) equal(other headers) bool {
	if len(*h) != len(other) {
		return false
	}
	for k, val1 := range *h {
		if val2 := other[k]; val1 != val2 {
			return false
		}
	}

	return true
}

func newSourceFile(fname string) (sf *sourceFile) {
	sf = &sourceFile{fname: fname, fpath: filepath.Join(opts.Source, fname)}
	sf.hdrs = headers{ContentType: mime.TypeByExtension(strings.ToLower(filepath.Ext(fname)))}

	for _, hdrs := range customHeadersDef {
		if hdrs.pathPattern.MatchString(fname) {
			sf.hdrs.merge(hdrs.headers)
			break
		}
	}
	sf.gzip = (sf.hdrs[ContentEncoding] == "gzip")

	return
}

func (s *sourceFile) getHeader(hdr string) *string {
	if hdr == Encryption {
		if opts.Encrypt {
			return &sse
		}

		return nil
	} else if v, ok := s.hdrs[hdr]; ok {
		return &v
	}

	return nil
}

func (s *sourceFile) recordAttempt() {
	s.Lock()
	s.attempts++
	s.Unlock()
}

func (s *sourceFile) retriable() bool {
	return s.attempts < maxTries
}
