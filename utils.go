package main

import (
	"bytes"
	"fmt"
	"mime"
	"path/filepath"
	"strings"
)

// S3 errors that we will retry.
var recoverableErrorsSuffixes = []string{
	"Idle connections will be closed.",
	"EOF",
	"broken pipe",
	"no such host",
	"transport closed before response was received",
	"TLS handshake timeout",
}

func loggerGen(buffers ...*bytes.Buffer) func(msgs ...string) {
	return func(msgs ...string) {
		m := msg(msgs...)

		if len(buffers) > 0 {
			buffers[0].WriteString(m)
			return
		}

		fmt.Print(m)
	}
}

// isRecoverable verifies if the error given is in recoverableErrorsSuffixes list.
func isRecoverable(err error) (yes bool) {
	for _, errSuffix := range recoverableErrorsSuffixes {
		if strings.HasSuffix(err.Error(), errSuffix) {
			return true
		}
	}

	return
}

// msg accepts 3 messages, corresponding to (in order): verbose, normal, quiet,
// and returns one of them based on the opts.verbose and opts.quiet flags.
//
// If the message for a respective state is blank, nothing will be printed,
// except if message for normal is missing. In that case, the verbose message
// will be printed if available.
func msg(msgs ...string) string {
	if opts.verbose && len(msgs) > 0 {
		return msgs[0] + "\n"
	} else if opts.quiet {
		if len(msgs) > 2 {
			return msgs[2]
		}
		return ""
	} else if len(msgs) > 1 {
		return msgs[1]
	}

	return ""
}

// betterMime wrapps mime.TypeByExtension and tries to handle a few edge cases.
func betterMime(fname string) (mt string) {
	ext := strings.ToLower(filepath.Ext(fname))
	if mt = mime.TypeByExtension(ext); mt != "" {
		return
	} else if ext == ".ttf" {
		mt = "binary/octet-stream"
	}

	return
}
