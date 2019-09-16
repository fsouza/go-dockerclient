.PHONY: \
	all \
	lint \
	fmt \
	fmtcheck \
	pretest \
	test \
	integration

all: test

lint:
	cd /tmp && GO111MODULE=on go get github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run

fmtcheck:
	if [ -z "$${SKIP_FMT_CHECK}" ]; then [ -z "$$(gofumpt -s -d . | tee /dev/stderr)" ]; fi

fmt:
	GO111MODULE=off go get mvdan.cc/gofumpt
	gofumpt -s -w .

testdeps:
	go mod download

pretest: lint fmtcheck

gotest:
	go test -race -vet all ./...

test: testdeps pretest gotest

integration:
	go test -tags docker_integration -run TestIntegration -v
