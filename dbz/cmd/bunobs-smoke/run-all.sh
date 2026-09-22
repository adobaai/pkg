#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$script_dir/../.."

run_case() {
  local name="$1"
  shift
  printf '\n=== %s ===\n' "$name"
  GOWORK=off go run ./cmd/bunobs-smoke "$@" 2>&1
}

run_case 'local, INFO' -env local -level INFO
run_case 'dev, DEBUG' -env dev -level DEBUG
run_case 'dev, INFO' -env dev -level INFO
run_case 'prod, DEBUG' -env prod -level DEBUG
run_case 'prod, INFO' -env prod -level INFO
run_case 'prod, DEBUG, query text enabled' -env prod -level DEBUG -query-text
run_case 'prod, INFO, query text enabled' -env prod -level INFO -query-text
run_case 'unknown environment, DEBUG' -env staging -level DEBUG
run_case 'dev, DEBUG, default logger' -env dev -level DEBUG -default-logger
