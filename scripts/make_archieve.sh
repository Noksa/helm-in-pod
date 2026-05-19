#!/usr/bin/env bash

# shellcheck disable=SC1091
source "$(dirname "$(realpath "$0")")/common.sh"

cyber_step "Building Release Archives"

rm -rf "${PROJECT_DIR}/generated"
mkdir -p "${PROJECT_DIR}/generated"
cd "${PROJECT_DIR}/generated"

version="$(grep "version" "${PROJECT_DIR}/plugin.yaml" | cut -d '"' -f 2)"
commit="$(git -C "${PROJECT_DIR}" rev-parse --short HEAD 2>/dev/null || echo "unknown")"
build_date="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
ldflags="-s -w -X github.com/noksa/helm-in-pod/cmd.version=${version} -X github.com/noksa/helm-in-pod/cmd.commit=${commit} -X github.com/noksa/helm-in-pod/cmd.date=${build_date}"
cyber_log "Version: ${CYBER_G}${version}${CYBER_X}"

TAR="tar"
if command -v gtar &>/dev/null; then
  TAR="gtar"
fi

ALL_ARCH="amd64 arm64"
ALL_OS="linux darwin windows"

# Support TARGET variable to build a single platform (e.g., TARGET=linux/amd64)
if [ -n "${TARGET:-}" ]; then
  IFS='/' read -r ALL_OS ALL_ARCH <<< "$TARGET"
  cyber_log "Target: ${CYBER_C}${TARGET}${CYBER_X}"
fi

for A in $ALL_ARCH; do
  for O in $ALL_OS; do
    output="in-pod"
    if [[ "$O" == "windows" ]]; then
      output="in-pod.exe"
    fi
    
    cyber_log "Building ${CYBER_C}${O}/${A}${CYBER_X}"
    CGO_ENABLED=0 GOARCH=$A GOOS=$O go build -ldflags "${ldflags}" -o "${output}" "${PROJECT_DIR}/main.go"
    
    archive="helm-in-pod_${version}_${O}_${A}.tar.gz"
    $TAR -czf "${archive}" "${output}"
    rm -rf "${output}"
    
    cyber_ok "Created ${CYBER_G}${archive}${CYBER_X}"
  done
done

cyber_ok "All archives created in ${CYBER_G}generated/${CYBER_X}"
