.PHONY: \
	all \
	deps \
	updatedeps \
	testdeps \
	updatetestdeps \
	lint \
	vet \
	errcheck \
	fmtcheck \
	pretest \
	test \
	cov \
	clean

all: test

deps:
	go get -d -v ./...

updatedeps:
	go get -d -v -u -f ./...

testdeps:
	go get -d -v -t ./...

updatetestdeps:
	go get -d -v -t -u -f ./...

lint: testdeps
	go get -v github.com/golang/lint/golint
	golint ./...

vet: testdeps
	go get -v golang.org/x/tools/cmd/vet
	go vet ./...

errcheck: testdeps
	go get -v github.com/kisielk/errcheck
	errcheck ./...

fmtcheck:
	for file in $(shell git ls-files '*.go'); do \
		gofmt $$file | diff -u $$file -; \
		if [ -n "$$(gofmt $$file | diff -u $$file -)" ]; then\
			exit 1; \
		fi; \
	done

# TODO(pedge): temporarily remove errcheck from requirements until all errors fixed
#pretest: lint vet errcheck fmtcheck
pretest: lint vet fmtcheck

test: testdeps pretest
	go test ./...

cov: testdeps
	go get -v github.com/axw/gocov/gocov
	go get golang.org/x/tools/cmd/cover
	gocov test | gocov report

clean:
	go clean ./...
