#!/usr/bin/env bash
set -euo pipefail

project_directory="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mygo_version="v0.6.0"
target_platforms="darwin/universal,windows/amd64"

cd "$project_directory"
go run "github.com/egoist/mygo/cmd/mygo@${mygo_version}" build \
  -platform "$target_platforms" -skip-notarize "$@"
