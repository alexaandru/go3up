go3up
=====

[![Build Status](https://travis-ci.org/alexaandru/go3up.png?branch=master)](https://travis-ci.org/alexaandru/go3up)
[![GoDoc](https://godoc.org/github.com/alexaandru/go3up?status.png)](https://godoc.org/github.com/alexaandru/go3up)
[![status](https://sourcegraph.com/api/repos/github.com/alexaandru/go3up/badges/status.png)](https://sourcegraph.com/github.com/alexaandru/go3up)

Go S3 Uploader

TODO
----

 - implement (optional) MD5 verification (see Content-MD5 at http://docs.aws.amazon.com/AmazonS3/latest/API/RESTObjectPUT.html);
 - implement multipart uploads + non "slurp mode" file read (+ make the limit where multipart kicks in configurable);
 - implement deletion of remote files missing on local;
 - (maybe) implement config file (from curr dir/home);
