#!/usr/bin/env bash
set -euo pipefail

root_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
source_dir=${RUNTIME_API_DIR:-"$root_dir/../mairo_competition_Backend/api/api"}
target_dir="$root_dir/contracts/runtime"

if [ ! -f "$source_dir/main.api" ]; then
  printf 'missing main.api: %s\n' "$source_dir" >&2
  exit 1
fi

rm -rf "$target_dir"
mkdir -p "$(dirname -- "$target_dir")"
cp -R "$source_dir" "$target_dir"

# Keep checked-in contracts stable across upstream editor line-ending choices.
find "$target_dir" -type f -exec perl -0777 -pi -e 's/\r\n/\n/g; s/ \t/\t/g; s/[ \t]+(?=\n)//g; s/\n+\z/\n/' {} +
