package go3up

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config mirrors the JSON-serializable Client settings, as stored in the config file.
type Config struct {
	BucketName   string `json:",omitempty"`
	Source       string `json:",omitempty"`
	CacheFile    string `json:",omitempty"`
	Region       string `json:",omitempty"`
	Profile      string `json:",omitempty"`
	WorkersCount int    `json:",omitempty"`
	Encrypt      bool   `json:",omitempty"`
}

// Options converts the non-zero Config fields into Client options.
func (cfg Config) Options() (opts []Option) {
	if cfg.BucketName != "" {
		opts = append(opts, WithBucket(cfg.BucketName))
	}

	if cfg.Source != "" {
		opts = append(opts, WithSource(cfg.Source))
	}

	if cfg.CacheFile != "" {
		opts = append(opts, WithCacheFile(cfg.CacheFile))
	}

	if cfg.Region != "" {
		opts = append(opts, WithRegion(cfg.Region))
	}

	if cfg.Profile != "" {
		opts = append(opts, WithProfile(cfg.Profile))
	}

	if cfg.WorkersCount > 0 {
		opts = append(opts, WithWorkers(cfg.WorkersCount))
	}

	if cfg.Encrypt {
		opts = append(opts, WithEncryption(true))
	}

	return
}

// Save writes the Config as indented JSON to the given file.
func (cfg Config) Save(fname string) (err error) {
	f, err := os.Create(fname) //nolint:gosec // fp
	if err != nil {
		return err
	}

	defer func() {
		err2 := f.Close()
		if err == nil {
			err = err2
		} else if err2 != nil {
			err = fmt.Errorf("%w; %w", err, err2)
		}
	}()

	buf, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}

	buf = append(buf, '\n')

	_, err = f.Write(buf)

	return
}

// LoadConfig reads a Config from the given file. A missing file yields a zero Config and no error.
func LoadConfig(fname string) (cfg Config, err error) {
	f, err := os.Open(fname) //nolint:gosec // ok
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}

		return Config{}, err
	}

	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	err = json.NewDecoder(f).Decode(&cfg)

	return
}
