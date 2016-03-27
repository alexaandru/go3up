package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type options struct {
	WorkersCount int    `json:",omitempty"`
	BucketName   string `json:",omitempty"`
	Source       string `json:",omitempty"`
	CacheFile    string `json:",omitempty"`
	Region       string `json:",omitempty"`
	Profile      string `json:",omitempty"`
	Encrypt      bool   `json:",omitempty"`

	dryRun, verbose, quiet,
	doCache, doUpload, saveCfg bool
	cfgFile string
}

func (o *options) dump(fname string) (err error) {
	f, err := os.Create(fname)
	if err != nil {
		return err
	}
	defer func() {
		err2 := f.Close()
		if err == nil {
			err = err2
		} else if err2 != nil {
			err = fmt.Errorf("%v; %v", err, err2)
		}
	}()

	var buf []byte
	buf, err = json.MarshalIndent(o, "", "  ")
	if err != nil {
		return
	}
	buf = append(buf, "\n"[0])

	_, err = f.Write(buf)

	return
}

func (o *options) restore(fname string) (err error) {
	f, err := os.Open(fname)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}
	defer func() {
		_ = f.Close()
	}()

	tmp := options{}
	dec := json.NewDecoder(f)
	if err = dec.Decode(&tmp); err != nil {
		return
	}

	o.merge(tmp)

	return nil
}

func (o *options) merge(other options) {
	if x := other.WorkersCount; x != 0 {
		o.WorkersCount = x
	}
	if x := other.BucketName; x != "" {
		o.BucketName = x
	}
	if x := other.Source; x != "" {
		o.Source = x
	}
	if x := other.CacheFile; x != "" {
		o.CacheFile = x
	}
	if x := other.Region; x != "" {
		o.Region = x
	}
	if x := other.Profile; x != "" {
		o.Profile = x
	}
	if x := other.Encrypt; x {
		o.Encrypt = x
	}

	// skipping the rest of the fields, they can never come from an unmarshalled file anyway.
}
