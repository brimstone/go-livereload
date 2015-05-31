.PHONY: small big static/build.go test

small: *.go static/build.go big
	${GOPATH}/bin/goupx go-livereload

big: *.go static/build.go
	go fmt
	go build

static/build.go:
	cd static; ${GOPATH}/bin/go-bindata -pkg=static -o=build.go -nomemcopy=true $(shell cd static; find . -type d)

test:
	sed -i "s/Date: .*</Date: $(shell date)</" test/index.html
