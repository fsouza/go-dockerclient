.PHONY: \
	all \
	vendor \
	cov \
	test \
	clean

all: test

vendor:
	go get -v github.com/mjibson/party
	party -d vendor -c -u

cov:
	go get -v github.com/axw/gocov/gocov
	go get golang.org/x/tools/cmd/cover
	gocov test | gocov report

test:
	go test ./.
	go test ./testing
	./testing/bin/fmtpolice

clean:
	go clean ./...
