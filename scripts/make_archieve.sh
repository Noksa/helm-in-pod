#!/usr/bin/env bash

# shellcheck disable=SC1091
source "$(dirname "$(realpath "$0")")/common.sh"

cyber_step "Building Release Archives"

rm -rf "${PROJECT_DIR}/generated"
mkdir -p "${PROJECT_DIR}/generated"
cd "${PROJECT_DIR}/generated"

version="$(grep "version" "${PROJECT_DIR}/plugin.yaml" | cut -d '"' -f 2)"
cyber_log "Version: ${CYBER_G}${version}${CYBER_X}"

ARCH="amd64 arm64"
OS="linux darwin windows"

for A in $ARCH; do
  for O in $OS; do
    output="in-pod"
    if [[ "$O" == "windows" ]]; then
      output="in-pod.exe"
    fi
    
    cyber_log "Building ${CYBER_C}${O}/${A}${CYBER_X}"
    CGO_ENABLED=0 GOARCH=$A GOOS=$O go build -o "${output}" "${PROJECT_DIR}/main.go"
    
    archive="helm-in-pod_${version}_${O}_${A}.tar.gz"
    gtar -czf "${archive}" "${output}"
    rm -rf "${output}"
    
    cyber_ok "Created ${CYBER_G}${archive}${CYBER_X}"
  done
done

cyber_ok "All archives created in ${CYBER_G}generated/${CYBER_X}"
