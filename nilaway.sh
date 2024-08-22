#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

echo "nilaway"
go install -v go.uber.org/nilaway/cmd/nilaway@latest
nilaway ./...
