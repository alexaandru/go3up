package main

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Headers
const (
	ContentEncoding      = "Content-Encoding"
	CacheControl         = "Cache-Control"
	ContentType          = "Content-Type"
	ServerSideEncryption = "x-amz-server-side-encryption"
)

// headers definition
type headersDef map[string]string

// headers in a format suitable for s3 put
type headers map[string]([]string)

type pathToHeaders struct {
	pathPattern *regexp.Regexp
	headers     map[string]string
}

type sourceFile struct {
	fname,
	fpath string
	hdrs     headers
	gzip     bool
	attempts int
	sync.Mutex
}

func (h *headers) merge(other headersDef) {
	for key, val := range other {
		(*h)[key] = []string{val}
	}
}

func (h *headers) equal(other headers) bool {
	if len(*h) != len(other) {
		return false
	}
	for k, val1 := range *h {
		val2 := other[k]
		if v1, v2 := strings.Join(val1, ":"), strings.Join(val2, ":"); v1 != v2 {
			return false
		}
	}

	return true
}

func newSourceFile(fname string) (sf *sourceFile) {
	sf = &sourceFile{fname: fname, fpath: filepath.Join(opts.source, fname)}
	sf.hdrs = headers{ContentType: {betterMime(fname)}}
	for _, hdrs := range customHeadersDef {
		if hdrs.pathPattern.MatchString(fname) {
			sf.hdrs.merge(hdrs.headers)
			break
		}
	}
	if gzip, ok := sf.hdrs[ContentEncoding]; ok {
		sf.gzip = (gzip[0] == "gzip")
	}
	if opts.encrypt {
		hdrs := headersDef{}
		hdrs[ServerSideEncryption] = "AES256"
		sf.hdrs.merge(hdrs)
	}

	return
}

func (s *sourceFile) body() (data []byte, err error) {
	data, err = ioutil.ReadFile(s.fpath)
	if err != nil {
		return
	}

	if s.gzip {
		buf := &bytes.Buffer{}
		w := gzip.NewWriter(buf)
		w.Write(data)
		err = w.Close()
		return buf.Bytes(), err
	}

	return
}

func (s *sourceFile) recordAttempt() {
	s.Lock()
	s.attempts++
	s.Unlock()
}

func (s *sourceFile) retriable() bool {
	return s.attempts < maxTries
}
