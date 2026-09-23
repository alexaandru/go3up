package go3up

import (
	"bytes"
	"errors"
	"testing"
)

func TestSay(t *testing.T) {
	buf := &bytes.Buffer{}
	c := newTestClient(t, WithLogger(newTestLogger(buf)))
	c.quiet = false

	c.say("verbose", "normal", "quiet")

	if actual := buf.String(); actual != "normal" {
		t.Error("Expected the logger to receive 'normal', got", actual)
	}
}

func TestIsRecoverable(t *testing.T) {
	c := newTestClient(t)

	tests := []struct {
		name string
		msg  string
		want bool
	}{
		{"not recoverable", "broken pipes all over", false},
		{"recoverable suffix", "Oh noes, I broken pipe", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := c.isRecoverable(errors.New(tc.msg)); got != tc.want {
				t.Errorf("Expected isRecoverable(%q) to be %v", tc.msg, tc.want)
			}
		})
	}
}

func TestMsg(t *testing.T) {
	c := newTestClient(t)
	c.verbose, c.quiet = false, false

	if actual := c.msg(); actual != "" {
		t.Error("Expected a blank message, got", actual)
	}

	c.verbose = true

	if actual := c.msg("Foo", "bar", "baz"); actual != "Foo\n" {
		t.Error("Expected Foo\\n got", actual)
	}

	c.quiet = true
	c.verbose = false

	if actual := c.msg("Foo", "bar", "baz"); actual != "baz" {
		t.Error("Expected baz got", actual)
	}

	if actual := c.msg(); actual != "" {
		t.Error("Expected message to be blank, got", actual)
	}

	c.quiet = false

	if actual := c.msg("Foo", "bar", "baz"); actual != "bar" {
		t.Error("Expected bar got", actual)
	}

	if actual := c.msg("Foo"); actual != "" {
		t.Error("Expected blank message got", actual)
	}
}
