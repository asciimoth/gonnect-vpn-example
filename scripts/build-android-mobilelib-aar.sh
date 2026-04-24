#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out_dir="${repo_root}/build/android"
out_aar="${out_dir}/mobilelib.aar"
go_cache_dir="${repo_root}/build/go"
gomobile_version="8baca8bf4eeba6bf71c89405abea7e4a00b63360"

mkdir -p "${out_dir}"
mkdir -p "${go_cache_dir}/gopath" "${go_cache_dir}/gomodcache" "${go_cache_dir}/gocache"

export ANDROID_HOME="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-}}"
export ANDROID_SDK_ROOT="${ANDROID_SDK_ROOT:-${ANDROID_HOME:-}}"
export ANDROID_NDK_HOME="${ANDROID_NDK_HOME:-${ANDROID_NDK_ROOT:-}}"
export ANDROID_NDK_ROOT="${ANDROID_NDK_ROOT:-${ANDROID_NDK_HOME:-}}"

if [[ -z "${ANDROID_SDK_ROOT}" ]]; then
  echo "ANDROID_SDK_ROOT is not set" >&2
  exit 1
fi

if [[ -z "${ANDROID_NDK_ROOT}" ]]; then
  echo "ANDROID_NDK_ROOT is not set" >&2
  exit 1
fi

if [[ ! -f "${ANDROID_NDK_ROOT}/meta/platforms.json" ]]; then
  ndk_candidate="$(find "${ANDROID_SDK_ROOT}/ndk" -mindepth 1 -maxdepth 1 -type l | sort | tail -n 1 || true)"
  if [[ -n "${ndk_candidate}" && -f "${ndk_candidate}/meta/platforms.json" ]]; then
    export ANDROID_NDK_HOME="${ndk_candidate}"
    export ANDROID_NDK_ROOT="${ndk_candidate}"
  fi
fi

if [[ ! -f "${ANDROID_NDK_ROOT}/meta/platforms.json" ]]; then
  echo "ANDROID_NDK_ROOT does not point to a usable Android NDK: ${ANDROID_NDK_ROOT}" >&2
  exit 1
fi

export GOPATH="${GOPATH:-${go_cache_dir}/gopath}"
export GOMODCACHE="${GOMODCACHE:-${go_cache_dir}/gomodcache}"
export GOCACHE="${GOCACHE:-${go_cache_dir}/gocache}"
export PATH="${GOPATH}/bin:${PATH}"
export GOFLAGS="${GOFLAGS:-} -modcacherw"

cd "${repo_root}"

go install "golang.org/x/mobile/cmd/gomobile@${gomobile_version}"
go install "golang.org/x/mobile/cmd/gobind@${gomobile_version}"

gomobile init
gomobile bind \
  -target=android \
  -androidapi 21 \
  -o "${out_aar}" \
  ./mobilelib

echo "Generated ${out_aar}"
