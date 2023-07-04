#!/usr/bin/env bash

rm -rf ../generated
mkdir -p ../generated
cd ../generated
version="$(cat ../plugin.yaml | grep "version" | cut -d '"' -f 2)"
ARCH="amd64 arm64"
OS="linux darwin windows"
for A in $ARCH; do
  for O in $OS; do
    output="in-pod"
    if [[ "$O" == "windows" ]]; then
      output="in-pod.exe"
    fi
    CGO_ENABLED=0 GOARCH=$A GOOS=$O go build -o "${output}" ../main.go
    gtar -czvf "helm-in-pod_${version}_${O}_${A}.tar.gz" "${output}"
    rm -rf "${output}"
  done
done