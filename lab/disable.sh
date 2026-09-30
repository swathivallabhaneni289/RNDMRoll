#!/usr/bin/env bash
# Removes the app/lab route shims created by lab/enable.sh. Refuses to delete
# anything it did not create.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST="$ROOT/app/lab"

if [ ! -e "$DEST" ]; then
  echo "Lab routes are not enabled."
  exit 0
fi

if ! grep -qs "lab/enable.sh" "$DEST/_layout.tsx"; then
  echo "app/lab was not created by lab/enable.sh; refusing to delete it." >&2
  exit 1
fi

rm -rf "$DEST"
echo "Lab routes removed."
