#!/usr/bin/env bash

mkdir -p ../generated
cd ../generated
version="$(cat ../plugin.yaml | grep "version" | cut -d '"' -f 2)"
ARCH="amd64 arm64"
OS="linux darwin windows"
for A in $ARCH; do
  for O in $OS; do
    output="helm-in-pod"
    if [[ "$O" == "windows" ]]; then
      output="helm-in-pod.exe"
    fi
    CGO_ENABLED=0 GOARCH=$A GOOS=$O go build -o "${output}" ../cmd/root.go
    tar -czvf "helm-in-pod_${version}_${O}_${A}.tar.gz" "${output}"
    rm -rf "${output}"
  done
done