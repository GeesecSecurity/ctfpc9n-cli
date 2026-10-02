#!/usr/bin/env bash
set -euo pipefail
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
source=${EVALUATION_API_SOURCE:-"$root/../backend/api/api/aiEvaluation.api"}
test -f "$source" || { echo "Set EVALUATION_API_SOURCE to main-platform aiEvaluation.api" >&2; exit 1; }
mkdir -p "$root/contracts/evaluation"
cp "$source" "$root/contracts/evaluation/main.api"
