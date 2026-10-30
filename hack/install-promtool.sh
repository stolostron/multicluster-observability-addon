#!/usr/bin/env bash

set -euo pipefail

version="${1:?Usage: install-promtool.sh VERSION DESTINATION}"
destination="${2:?Usage: install-promtool.sh VERSION DESTINATION}"

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *)
    echo "Unsupported operating system: $(uname -s)" >&2
    exit 1
    ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *)
    echo "Unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

package="prometheus-${version}.${os}-${arch}"
archive="${package}.tar.gz"
release="https://github.com/prometheus/prometheus/releases/download/v${version}"
download_dir=$(mktemp -d)
trap 'rm -rf "$download_dir"' EXIT

curl --fail --location --silent --show-error --retry 3 "${release}/${archive}" -o "${download_dir}/${archive}"
curl --fail --location --silent --show-error --retry 3 "${release}/sha256sums.txt" -o "${download_dir}/sha256sums.txt"

# Verify the release archive before extracting the test tool.
checksum=$(awk -v archive="$archive" '$2 == archive { print; found = 1 } END { exit !found }' "${download_dir}/sha256sums.txt")
(
  cd "$download_dir"
  if command -v sha256sum >/dev/null 2>&1; then
    printf '%s\n' "$checksum" | sha256sum -c -
  else
    printf '%s\n' "$checksum" | shasum -a 256 -c -
  fi
)

tar -xzf "${download_dir}/${archive}" -C "$download_dir" "${package}/promtool"
mkdir -p "$(dirname "$destination")"
install -m 0755 "${download_dir}/${package}/promtool" "$destination"
