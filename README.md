go-livereload
=============

Single binary that implements a webserver, livereload server, and watches every
file and directory in the current directory for changes.

Usage
-----
1. Put go-livereload in your path
1. `go-livereload`
1. Open http://localhost:8000
1. Edit files
1. Enjoy live reloading pages

Build
-----
If you don't want to change the version of livereload.js embedded in the binary,
simply, `go install -v github.com/brimstone/go-livereload`

If you do, check out the source and `make big` or `make small`
