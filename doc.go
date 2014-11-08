/*
Go3Up (Go S3 Uploader) is a small S3 uploader tool.

It was created in order to speed up S3 uploads by employing a local caching of files' md5 sums.
That way, on subsequent runs, go3up can compute a list of the files that changed since the
last upload and only upload those.

The initial use case was a large static site (with 10k+ files) that frequently changed only
a small subset of files (about ~100 routinely). In that particular case, the time reduction by
switching from s3cmd to go3up was significant.

On uploads with empty cache there may not be any benefit.

The current focus of the tool is just one way/uploads (without deleting things that were removed
locally, yet). That may (or not) change in the future.
*/
package main
