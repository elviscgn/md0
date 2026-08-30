#!/bin/sh
set -eu

repository="elviscgn/md0"
install_dir="${INSTALL_DIR:-/usr/local/bin}"
version="${MD0_VERSION:-latest}"

case "$(uname -s)" in
  Linux) os="linux" ;;
  Darwin) os="darwin" ;;
  *) echo "md0: unsupported operating system: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "md0: unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

archive="md0-${os}-${arch}.tar.gz"
if [ "$version" = "latest" ]; then
  release_url="https://github.com/${repository}/releases/latest/download"
else
  release_url="https://github.com/${repository}/releases/download/${version}"
fi

temp_dir="$(mktemp -d "${TMPDIR:-/tmp}/md0-install.XXXXXX")"
trap 'rm -rf "$temp_dir"' EXIT HUP INT TERM

echo "Downloading ${archive}..."
curl -fL --proto '=https' --tlsv1.2 "${release_url}/${archive}" -o "${temp_dir}/${archive}"
curl -fL --proto '=https' --tlsv1.2 "${release_url}/SHA256SUMS.txt" -o "${temp_dir}/SHA256SUMS.txt"

expected="$(awk -v file="$archive" '$2 == file { print $1 }' "${temp_dir}/SHA256SUMS.txt")"
if [ -z "$expected" ]; then
  echo "md0: release checksum is missing ${archive}" >&2
  exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "${temp_dir}/${archive}" | awk '{ print $1 }')"
else
  actual="$(shasum -a 256 "${temp_dir}/${archive}" | awk '{ print $1 }')"
fi
if [ "$actual" != "$expected" ]; then
  echo "md0: checksum verification failed for ${archive}" >&2
  exit 1
fi

tar -xzf "${temp_dir}/${archive}" -C "$temp_dir"
mkdir -p "$install_dir"
cp "${temp_dir}/md0" "${install_dir}/md0"
chmod 0755 "${install_dir}/md0"
echo "Installed md0 to ${install_dir}/md0"
