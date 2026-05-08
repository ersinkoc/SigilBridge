#!/usr/bin/env bash
set -euo pipefail

version="${1:-v1.0.0}"
goos="${2:-linux}"
goarch="${3:-amd64}"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
work="${root}/packaging/deb/work"
pkg="${work}/sigilbridge_${version#v}_${goarch}"
artifact="${root}/dist/sigilbridge_${version}_${goos}_${goarch}.tar.gz"

if [[ ! -f "${artifact}" ]]; then
  echo "missing artifact: ${artifact}" >&2
  exit 1
fi

rm -rf "${work}"
mkdir -p "${pkg}/DEBIAN" "${pkg}/usr/local/bin" "${pkg}/etc/sigilbridge" "${pkg}/lib/systemd/system"

tar -C "${work}" -xzf "${artifact}"
cp "${work}/sigilbridge_${version}_${goos}_${goarch}/sigilbridge" "${pkg}/usr/local/bin/sigilbridge"
cp "${root}/examples/config.yaml" "${pkg}/etc/sigilbridge/config.yaml.example"
cp "${root}/examples/pools.yaml" "${pkg}/etc/sigilbridge/pools.yaml.example"
cp "${root}/deployments/systemd/sigilbridge.service" "${pkg}/lib/systemd/system/sigilbridge.service"

sed "s/@VERSION@/${version#v}/g; s/@ARCH@/${goarch}/g" \
  "${root}/packaging/deb/control" > "${pkg}/DEBIAN/control"
cp "${root}/packaging/deb/postinst" "${pkg}/DEBIAN/postinst"
cp "${root}/packaging/deb/prerm" "${pkg}/DEBIAN/prerm"
chmod 0755 "${pkg}/DEBIAN/postinst" "${pkg}/DEBIAN/prerm" "${pkg}/usr/local/bin/sigilbridge"

dpkg-deb --build "${pkg}" "${root}/dist/sigilbridge_${version#v}_${goarch}.deb"
