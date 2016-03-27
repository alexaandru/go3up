package main

import (
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
)

func TestFilesList(t *testing.T) {
	cacheFile := opts.CacheFile
	opts.CacheFile = "test/.cacheEmpty.txt"
	current, diff := filesLists()

	if current["barbaz.txt"] != "dac2e8bd758efb58a30f9fcd7ac28b1b" ||
		current["foobar.html"] != "01677e4c0ae5468b9b8b823487f14524" {
		t.Error("Current list does not match expectation")
	}

	sort.Strings(diff)
	if strings.Join(diff, ":") != "barbaz.txt:foobar.html" {
		t.Error("Expected diff to hold barbaz.txt and foobar.html")
	}

	opts.CacheFile = cacheFile
}

func TestUpload(t *testing.T) {
	upFn, uploads := fakeUploaderGen()
	up := make(chan *sourceFile)
	rejected := &syncedlist{}
	wgUploads, wgWorkers := new(sync.WaitGroup), new(sync.WaitGroup)

	wgUploads.Add(2)
	wgWorkers.Add(1)

	opts.verbose = false
	opts.quiet = true
	go upload("a", upFn, up, rejected, wgUploads, wgWorkers)

	up <- newSourceFile("foobar.html")
	up <- newSourceFile("barbaz.txt")

	wgUploads.Wait()
	close(up)
	wgWorkers.Wait()
	opts.quiet = false

	if len(*uploads) != 2 {
		t.Fatal("Expected to upload 2 files, got", *uploads)
	}
}

func TestUploadDryRun(t *testing.T) {
	upFn, uploads := fakeUploaderGen()
	up := make(chan *sourceFile)
	rejected := &syncedlist{}
	wgUploads, wgWorkers := new(sync.WaitGroup), new(sync.WaitGroup)

	wgUploads.Add(2)
	wgWorkers.Add(1)

	origDry := opts.dryRun
	opts.dryRun = true
	opts.verbose = false
	opts.quiet = true
	go upload("b", upFn, up, rejected, wgUploads, wgWorkers)

	up <- newSourceFile("foobar.html")
	up <- newSourceFile("barbaz.txt")

	wgUploads.Wait()
	close(up)
	wgWorkers.Wait()
	opts.dryRun = origDry
	opts.quiet = false

	if len(*uploads) > 0 {
		t.Fatal("Expected to get a blank uploads list, got", *uploads)
	}
}

func TestUploadUnrecoverable(t *testing.T) {
	upFn, uploads := fakeUploaderGen(fatalError)
	up := make(chan *sourceFile)
	rejected := &syncedlist{}
	wgUploads, wgWorkers := new(sync.WaitGroup), new(sync.WaitGroup)

	wgUploads.Add(2)
	wgWorkers.Add(1)

	opts.verbose = false
	opts.quiet = true
	go upload("c", upFn, up, rejected, wgUploads, wgWorkers)

	up <- newSourceFile("foobar.html")
	up <- newSourceFile("barbaz.txt")

	wgUploads.Wait()
	close(up)
	wgWorkers.Wait()
	opts.quiet = false

	if len(*uploads) != 2 {
		t.Fatal("Expected both uploads to be processed, got", *uploads)
	}
	if len(rejected.list) != 2 {
		t.Fatal("Expected all of the uploads to be rejected, got", rejected.list)
	}
}

func TestUploadRecoverable(t *testing.T) {
	upFn, uploads := fakeUploaderGen(recoverableError)
	_ = uploads
	up := make(chan *sourceFile)
	rejected := &syncedlist{}
	wgUploads, wgWorkers := new(sync.WaitGroup), new(sync.WaitGroup)

	wgUploads.Add(2)
	wgWorkers.Add(2)

	opts.quiet = true
	opts.verbose = false
	go upload("d", upFn, up, rejected, wgUploads, wgWorkers)
	go upload("e", upFn, up, rejected, wgUploads, wgWorkers)

	sf1, sf2 := newSourceFile("barbaz.txt"), newSourceFile("foobar.html")
	up <- sf1
	up <- sf2

	wgUploads.Wait()
	close(up)
	wgWorkers.Wait()
	opts.quiet = false

	if lu := len(*uploads); lu != 2*maxTries {
		t.Fatal("Expected both uploads to be processed maxTries, got", lu, "attempts")
	}
	if sf1.attempts != maxTries || sf2.attempts != maxTries {
		t.Fatal("Expected both files to have their attempts exhausted got", sf1.attempts, "and", sf2.attempts)
	}
	if len(rejected.list) != 2 {
		t.Fatal("Expected all of the uploads to be rejected, got", rejected.list)
	}
}

func TestIntegrationMain(t *testing.T) {
	if _, err := os.Create(opts.CacheFile); err != nil {
		t.Fatal("Failed to truncate the cache file")
	}

	upFn, uploads := fakeUploaderGen()
	_ = upFn
	opts.Region = "us-west-1"
	opts.quiet = true
	main()
	opts.quiet = false

	fnames := make([]string, len(*uploads))
	for k, v := range *uploads {
		fnames[k] = v.fname
	}
	sort.Strings(fnames)
	t.Skip("Not really tested for now, other than seeing that it does not break. Will need some assertions.")
	// if expected, actual := "barbaz.txt:foobar.html", strings.Join(fnames, ":"); expected != actual {
	// 	t.Fatalf("Expected %s to be uploaded got %s", expected, actual)
	// }
}

func TestIntegrationPartialUpload(t *testing.T) {
	t.Skip()
}
