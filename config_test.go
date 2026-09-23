package go3up

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestOptions(t *testing.T) {
	t.Run("all fields", func(t *testing.T) {
		cfg := Config{
			BucketName:   "example_bucket",
			Source:       "testdata/output",
			CacheFile:    "testdata/.go3up.txt",
			Region:       "us-west-1",
			Profile:      "test",
			WorkersCount: 7,
			Encrypt:      true,
		}

		c, err := New(cfg.Options()...)
		if err != nil {
			t.Fatal("Failed to create client:", err)
		}

		if c.bucketName != "example_bucket" || c.source != "testdata/output" || c.cacheFile != "testdata/.go3up.txt" ||
			c.region != "us-west-1" || c.profile != "test" || c.workersCount != 7 || !c.encrypt {
			t.Errorf("Config options not applied: %+v", c)
		}
	})

	t.Run("zero fields skipped", func(t *testing.T) {
		if opts := (Config{}).Options(); len(opts) != 0 {
			t.Error("Expected no options from a zero Config, got", len(opts))
		}
	})
}

func TestSave(t *testing.T) {
	t.Run("roundtrip", func(t *testing.T) {
		fname := filepath.Join(t.TempDir(), "cfg.json")
		want := Config{BucketName: "example_bucket", Source: "src", WorkersCount: 3}

		if err := want.Save(fname); err != nil {
			t.Fatal("Save failed:", err)
		}

		got, err := LoadConfig(fname)
		if err != nil {
			t.Fatal("LoadConfig failed:", err)
		}

		if !reflect.DeepEqual(want, got) {
			t.Errorf("Expected %+v, got %+v", want, got)
		}
	})

	t.Run("unwritable path", func(t *testing.T) {
		if err := (Config{}).Save(filepath.Join(t.TempDir(), "bogus", "cfg.json")); err == nil {
			t.Error("Expected Save to fail on a missing directory")
		}
	})
}

func TestLoadConfig(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		cfg, err := LoadConfig(filepath.Join(t.TempDir(), "missing.json"))
		if err != nil {
			t.Fatal("Expected a missing file to be tolerated, got", err)
		}

		if cfg != (Config{}) {
			t.Error("Expected a zero Config, got", cfg)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		fname := filepath.Join(t.TempDir(), "cfg.json")
		if err := os.WriteFile(fname, []byte("{bogus"), 0o644); err != nil {
			t.Fatal(err)
		}

		if _, err := LoadConfig(fname); err == nil {
			t.Error("Expected invalid JSON to fail")
		}
	})

	t.Run("unreadable path", func(t *testing.T) {
		if _, err := LoadConfig(t.TempDir()); err == nil {
			t.Error("Expected reading a directory to fail")
		}
	})
}
