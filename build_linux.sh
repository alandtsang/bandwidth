#!/usr/bin/env bash
set -e
cd "$(dirname "$0")"

if [ "$(uname)" == "Darwin" ]; then
	export GOOS="${GOOS:-linux}"
fi

export GOFLAGS="${GOFLAGS} -mod=vendor"

mkdir -p "${PWD}/bin"

echo "Building plugins bandwidth"
go build -mod=vendor -o "${PWD}/bin/bandwidth" main.go