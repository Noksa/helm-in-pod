#!/usr/bin/env sh

set -e

cd $HELM_PLUGIN_DIR
version="$(cat plugin.yaml | grep "version" | cut -d '"' -f 2)"
echo "Installing helm-in-pod v${version} ..."

unameOut="$(uname -s)"

case "${unameOut}" in
    Linux*)     os=linux;;
    Darwin*)    os=darwin;;
#    CYGWIN*)    os=cygwin;;
    MINGW*)     os=windows;;
    *)          os="UNKNOWN:${unameOut}"
esac

arch=$(uname -m)

if [ "$arch" = "x86_64" ]; then
  arch="amd64"
else
  arch="arm64"
fi

url="https://github.com/Noksa/helm-in-pod/releases/download/v${version}/helm-in-pod_${version}_${os}_${arch}.tar.gz"

if [ "$url" = "" ]; then
  echo "Unsupported OS / architecture: ${os}_${arch}"
  exit 1
fi

filename="helm-in-pod_${version}.tar.gz"


if [ -z "$(command -v tar)" ]; then
  echo "tar is required, install it first"
  exit 1
fi

# Download archive
download_failed=false
if [ -n "$(command -v curl)" ]; then
  http_code=$(curl -sSL -o "$filename" -w "%{http_code}" "$url")
  if [ "$http_code" != "200" ]; then
    download_failed=true
    rm -f "$filename"
  fi
elif [ -n "$(command -v wget)" ]; then
  if ! wget -q "$url" -O "$filename"; then
    download_failed=true
    rm -f "$filename"
  fi
else
  echo "Need curl or wget"
  exit 1
fi

# Fallback to latest release if the requested version is not available yet
if [ "$download_failed" = "true" ]; then
  echo "WARNING: v${version} release not found (may still be building). Falling back to latest release..."
  if [ -n "$(command -v curl)" ]; then
    latest_version=$(curl -sSL -o /dev/null -w "%{url_effective}" "https://github.com/Noksa/helm-in-pod/releases/latest" | grep -o '[^/]*$' | sed 's/^v//')
  else
    latest_version=$(wget --max-redirect=0 "https://github.com/Noksa/helm-in-pod/releases/latest" 2>&1 | grep -i "Location" | grep -o '[^/]*$' | sed 's/^v//')
  fi
  if [ -z "$latest_version" ]; then
    echo "Failed to determine latest release version"
    exit 1
  fi
  latest_url="https://github.com/Noksa/helm-in-pod/releases/download/v${latest_version}/helm-in-pod_${latest_version}_${os}_${arch}.tar.gz"
  if [ -n "$(command -v curl)" ]; then
    http_code=$(curl -sSL -o "$filename" -w "%{http_code}" "$latest_url")
    if [ "$http_code" != "200" ]; then
      echo "Failed to download latest release (v${latest_version}) as well"
      exit 1
    fi
  else
    if ! wget -q "$latest_url" -O "$filename"; then
      echo "Failed to download latest release (v${latest_version}) as well"
      exit 1
    fi
  fi
  echo "WARNING: Installed v${latest_version} instead of v${version}. Re-run 'helm plugin update in-pod' in a few minutes to get v${version}."
  installed_version="$latest_version"
fi

trap 'rm -rf $filename' EXIT

# Install bin
rm -rf bin && mkdir bin && tar xzvf "$filename" -C bin > /dev/null && rm -f "$filename"

if [ "$?" != "0" ]; then
  echo "an error has occured"
  exit 1
fi

echo "helm-in-pod ${installed_version:-${version}} has been installed"
echo
echo "Check https://github.com/Noksa/helm-in-pod for usage"
