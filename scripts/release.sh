#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist"
VERSION="${VERSION:-${GITHUB_REF_NAME:-dev}}"
COMMIT="${COMMIT:-$(git -C "${ROOT_DIR}" rev-parse --short=12 HEAD 2>/dev/null || echo unknown)}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
go_bin="go"
if ! command -v "${go_bin}" >/dev/null 2>&1; then
  if command -v go.exe >/dev/null 2>&1; then
    go_bin="go.exe"
  else
    echo "go is required to build release artifacts" >&2
    exit 1
  fi
fi

targets=(
  "linux amd64"
  "linux arm64"
  "darwin amd64"
  "darwin arm64"
  "windows amd64"
)

rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

if [[ -f "${ROOT_DIR}/ui/package.json" && "${SKIP_UI_BUILD:-0}" != "1" ]]; then
  pnpm_bin=""
  if command -v pnpm >/dev/null 2>&1 && pnpm --version >/dev/null 2>&1; then
    pnpm_bin="pnpm"
  elif command -v pnpm.cmd >/dev/null 2>&1 && pnpm.cmd --version >/dev/null 2>&1; then
    pnpm_bin="pnpm.cmd"
  else
    echo "pnpm is required to build the embedded UI" >&2
    exit 1
  fi
  "${pnpm_bin}" --dir "${ROOT_DIR}/ui" install --frozen-lockfile
  "${pnpm_bin}" --dir "${ROOT_DIR}/ui" run build
  rm -rf "${ROOT_DIR}/internal/admin/ui/dist"
  mkdir -p "${ROOT_DIR}/internal/admin/ui/dist"
  cp -R "${ROOT_DIR}/ui/dist/." "${ROOT_DIR}/internal/admin/ui/dist/"
elif [[ "${SKIP_UI_BUILD:-0}" == "1" ]]; then
  echo "skipping UI build because SKIP_UI_BUILD=1"
fi

for target in "${targets[@]}"; do
  read -r goos goarch <<<"${target}"
  name="sigilbridge_${VERSION}_${goos}_${goarch}"
  out_dir="${DIST_DIR}/${name}"
  mkdir -p "${out_dir}"

  binary="sigilbridge"
  if [[ "${goos}" == "windows" ]]; then
    binary="sigilbridge.exe"
  fi

  echo "building ${name}"
  (
    cd "${ROOT_DIR}"
    GOOS="${goos}" GOARCH="${goarch}" CGO_ENABLED=0 "${go_bin}" build \
      -trimpath \
      -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_DATE}" \
      -o "${out_dir}/${binary}" ./cmd/sigilbridge
  )

  cp "${ROOT_DIR}/README.md" "${out_dir}/README.md"
  cp "${ROOT_DIR}/LICENSE" "${out_dir}/LICENSE"
  cp "${ROOT_DIR}/examples/config.yaml" "${out_dir}/config.example.yaml"
  cp "${ROOT_DIR}/examples/pools.yaml" "${out_dir}/pools.example.yaml"

  tar -C "${DIST_DIR}" -czf "${DIST_DIR}/${name}.tar.gz" "${name}"
  rm -rf "${out_dir}"
done

(
  cd "${DIST_DIR}"
  sha256sum *.tar.gz > checksums.txt
)

if [[ "${PUBLISH_DOCKER:-0}" == "1" ]] && command -v docker >/dev/null 2>&1; then
  image="${DOCKER_IMAGE:-ghcr.io/sigilbridge/sigilbridge}"
  docker buildx build \
    --platform linux/amd64,linux/arm64 \
    --build-arg VERSION="${VERSION}" \
    --build-arg COMMIT="${COMMIT}" \
    --build-arg BUILD_DATE="${BUILD_DATE}" \
    -t "${image}:${VERSION}" \
    -t "${image}:latest" \
    --push \
    -f "${ROOT_DIR}/deployments/docker/Dockerfile" \
    "${ROOT_DIR}"
fi

if [[ -n "${GITHUB_TOKEN:-}" ]] && command -v gh >/dev/null 2>&1 && [[ "${VERSION}" == v* ]]; then
  gh release create "${VERSION}" "${DIST_DIR}"/*.tar.gz "${DIST_DIR}/checksums.txt" \
    --title "SigilBridge ${VERSION}" \
    --notes "Release ${VERSION}" \
    --verify-tag || \
  gh release upload "${VERSION}" "${DIST_DIR}"/*.tar.gz "${DIST_DIR}/checksums.txt" --clobber
fi

echo "release artifacts written to ${DIST_DIR}"
