#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

echo "clean"
rm -f coverage.out go.work.sum
echo "lint"
go install -v github.com/go-critic/go-critic/cmd/gocritic@latest
gocritic check -enableAll ./...
echo "nilaway"
go install -v go.uber.org/nilaway/cmd/nilaway@latest
nilaway ./...
echo "format"
gofmt -e -l -s -w .
echo "vulnerability check"
go install -v golang.org/x/vuln/cmd/govulncheck@latest
govulncheck -show verbose ./...
echo "unit tests"
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out