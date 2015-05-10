.PHONY: \
	all \
	vendor \
	cov \
	test \
	clean

all: test

vendor:
	go get -v github.com/kardianos/vendor
	vendor add -status external
	vendor update -status external

cov: testdeps
	go get -v github.com/axw/gocov/gocov
	go get golang.org/x/tools/cmd/cover
	gocov test | gocov report

test: testdeps
	go test ./...
	./testing/bin/fmtpolice

clean:
	go clean ./...
