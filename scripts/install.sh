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

url="https://github.com/Noksa/helm-in-pod/releases/download/${version}/helm-in-pod_${version}_${os}_${arch}.tar.gz"

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
if [ -n "$(command -v curl)" ]; then
  curl -sSL "$url" -o "$filename"
elif [ -n "$(command -v wget)" ]; then
  wget -q "$url" -o "$filename"
else
  echo "Need curl or wget"
  exit 1
fi

trap 'rm -rf $filename' EXIT

# Install bin
rm -rf bin && mkdir bin && tar xzvf "$filename" -C bin > /dev/null && rm -f "$filename"

if [ "$?" != "0" ]; then
  echo "an error has occured"
  exit 1
fi

echo "helm-in-pod ${version} has been installed"
echo
echo "Check https://github.com/Noksa/helm-in-pod for usage"
