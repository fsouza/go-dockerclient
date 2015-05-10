.PHONY: \
	all \
	vendor \
	lint \
	vet \
	fmt \
	fmtcheck \
	pretest \
	test \
	cov \
	clean

all: test

vendor:
	go get -v github.com/mjibson/party
	party -c -u

lint:
	go get -v github.com/golang/lint/golint
	for file in $(shell git ls-files '*.go' | grep -v '^_third_party/'); do \
		golint $$file; \
	done

vet:
	go get -v golang.org/x/tools/cmd/vet
	go vet ./...

fmt:
	gofmt -w $(shell git ls-files '*.go' | grep -v '^_third_party/')

fmtcheck:
	for file in $(shell git ls-files '*.go' | grep -v '^_third_party/'); do \
		gofmt $$file | diff -u $$file -; \
		if [ -n "$$(gofmt $$file | diff -u $$file -)" ]; then\
			exit 1; \
		fi; \
	done

pretest: lint vet fmtcheck

test: pretest
	go test ./...

cov:
	go get -v github.com/axw/gocov/gocov
	go get golang.org/x/tools/cmd/cover
	gocov test | gocov report

clean:
	go clean ./...
