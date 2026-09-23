package go3up

import (
	"fmt"
	"strings"
)

func defaultLogger(msg string) {
	fmt.Print(msg)
}

// defaultRecoverableErrors returns the S3 error suffixes that we will retry.
func defaultRecoverableErrors() []string {
	return []string{
		"Idle connections will be closed.",
		"EOF",
		"broken pipe",
		"no such host",
		"transport closed before response was received",
		"TLS handshake timeout",
	}
}

// say formats one of the (verbose, normal, quiet) messages and passes it to the client's logger.
func (c *Client) say(msgs ...string) {
	c.logger(c.msg(msgs...))
}

// isRecoverable verifies if the error given is in the recoverable errors list.
func (c *Client) isRecoverable(err error) (yes bool) {
	for _, errSuffix := range c.recoverableErrors {
		if strings.HasSuffix(err.Error(), errSuffix) {
			return true
		}
	}

	return
}

// msg accepts 3 messages, corresponding to (in order): verbose, normal, quiet,
// and returns one of them based on the verbose and quiet flags.
//
// If the message for a respective state is blank, nothing will be printed,
// except if message for normal is missing. In that case, the verbose message
// will be printed if available.
func (c *Client) msg(msgs ...string) (_ string) {
	switch {
	case c.verbose && len(msgs) > 0:
		return msgs[0] + "\n"
	case c.quiet:
		if len(msgs) > 2 {
			return msgs[2]
		}

		return
	case len(msgs) > 1:
		return msgs[1]
	default:
		return
	}
}
