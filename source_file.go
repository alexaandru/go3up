package go3up

import (
	"maps"
	"mime"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

type headers map[string]string

// HeaderRule maps a path pattern to a set of headers. First match wins.
type HeaderRule struct {
	PathPattern *regexp.Regexp
	Headers     map[string]string
}

type sourceFile struct {
	hdrs     headers
	fname    string
	fpath    string
	attempts int
	sync.Mutex
	gzip    bool
	encrypt bool
}

const (
	ContentEncoding = "Content-Encoding"
	CacheControl    = "Cache-Control"
	ContentType     = "Content-Type"
	Encryption      = "EncryptionON" //  pseudo header enabling SSE-S3 encryption
)

const (
	sse = "AES256"
	gz  = "gzip"
)

// defaultHeaderRules returns the default path to headers mappings.
// Order matters: first hit, first served.
func defaultHeaderRules() []HeaderRule {
	r := regexp.MustCompile

	return []HeaderRule{
		{r("index\\.html"), map[string]string{ContentEncoding: gz, CacheControl: "max-age=1800"}},       // 1800.
		{r("articole.*\\.html$"), map[string]string{ContentEncoding: gz, CacheControl: "max-age=3600"}}, // 86400.
		{r("[^/]*\\.html$"), map[string]string{ContentEncoding: gz, CacheControl: "max-age=3600"}},
		{r("\\.xml$"), map[string]string{ContentEncoding: gz, CacheControl: "max-age=1800"}},
		{r("\\.ico$"), map[string]string{ContentEncoding: gz, CacheControl: "max-age=31536000"}},
		{r("\\.(js|css)$"), map[string]string{ContentEncoding: gz, CacheControl: "max-age=31536000"}},
		{r("images/articole/.*(jpg|JPG|png|PNG)$"), map[string]string{CacheControl: "max-age=31536000"}},
		{r("\\.(jpg|JPG|png|PNG)$"), map[string]string{CacheControl: "max-age=31536000"}},
	}
}

func (h *headers) merge(other map[string]string) {
	maps.Copy((*h), other)
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

func (c *Client) newSourceFile(fname string) (sf *sourceFile) {
	sf = &sourceFile{fname: fname, fpath: filepath.Join(c.source, fname), encrypt: c.encrypt}
	sf.hdrs = headers{ContentType: mime.TypeByExtension(strings.ToLower(filepath.Ext(fname)))}

	for _, rule := range c.headerRules {
		if rule.PathPattern.MatchString(fname) {
			sf.hdrs.merge(rule.Headers)
			break
		}
	}

	sf.gzip = (sf.hdrs[ContentEncoding] == gz)

	return
}

func (s *sourceFile) getHeader(hdr string) *string {
	if hdr == Encryption {
		if s.encrypt {
			v := sse

			return &v
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
