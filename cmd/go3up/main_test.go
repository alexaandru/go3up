package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/alexaandru/go3up"
)

func TestRun(t *testing.T) {
	setup := func(t *testing.T) (source, cache string) {
		t.Helper()

		dir := t.TempDir()
		source = filepath.Join(dir, "output")
		cache = filepath.Join(dir, ".go3up.txt")

		if err := os.Mkdir(source, 0o755); err != nil {
			t.Fatal(err)
		}

		if _, err := os.Create(cache); err != nil {
			t.Fatal(err)
		}

		return
	}

	t.Run("bad flag", func(t *testing.T) {
		if code := run([]string{"-bogus"}, io.Discard); code != CmdLineOptionError {
			t.Error("Expected CmdLineOptionError, got", code)
		}
	})

	t.Run("missing bucket", func(t *testing.T) {
		source, cache := setup(t)
		args := []string{"-source", source, "-cachefile", cache}

		if code := run(args, io.Discard); code != CmdLineOptionError {
			t.Error("Expected CmdLineOptionError, got", code)
		}
	})

	t.Run("bad cfgfile", func(t *testing.T) {
		args := []string{"-cfgfile", t.TempDir()} //  a directory, not a JSON file

		if code := run(args, io.Discard); code != SetupFailed {
			t.Error("Expected SetupFailed, got", code)
		}
	})

	t.Run("dry run succeeds", func(t *testing.T) {
		source, cache := setup(t)
		args := []string{"-bucket", "example_bucket", "-source", source, "-cachefile", cache, "-dry", "-quiet"}

		if code := run(args, io.Discard); code != Success {
			t.Error("Expected Success, got", code)
		}
	})

	t.Run("save config", func(t *testing.T) {
		source, cache := setup(t)
		cfgFile := filepath.Join(t.TempDir(), "cfg.json")

		// -cfgfile reloads the config after flag parsing, so seed it with the required fields.
		cfg := go3up.Config{BucketName: "example_bucket", Source: source, CacheFile: cache}
		if err := cfg.Save(cfgFile); err != nil {
			t.Fatal("Failed to seed config file:", err)
		}

		args := []string{"-cfgfile", cfgFile, "-save", "-dry", "-quiet"}

		if code := run(args, io.Discard); code != Success {
			t.Error("Expected Success, got", code)
		}

		if _, err := os.Stat(cfgFile); err != nil {
			t.Error("Expected config file to be saved:", err)
		}
	})
}
