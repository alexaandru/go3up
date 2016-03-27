# Go S3 Uploader

[![Build Status](https://travis-ci.org/alexaandru/go3up.png?branch=master)](https://travis-ci.org/alexaandru/go3up)
[![GoDoc](https://godoc.org/github.com/alexaandru/go3up?status.png)](https://godoc.org/github.com/alexaandru/go3up)

Go3Up (Go S3 Uploader) is a small S3 uploader tool.

It was created in order to speed up S3 uploads by employing a local caching of files' md5 sums.
That way, on subsequent runs, go3up can compute a list of the files that changed since the
last upload and only upload those.

The initial use case was a large static site (with 10k+ files) that frequently changed only
a small subset of files (about ~100 routinely). In that particular case, the time reduction by
switching from s3cmd to go3up was significant.

On uploads with empty cache there may not be any benefit.

The current focus of the tool is just one way/uploads (without deleting things that were removed
locally, yet).

## Usage

Run `go3up -h` to get the help. You can save your preferences to a .go3up.json config file by
passing your command line flags as usual and adding "-save" at the end.

For authentication, see http://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html
as we pretty much support all of those options, in this order: shared profile; EC2 role; env vars.

## TODO

 - implement (optional) deletion of remote files missing on local.
