#!/usr/bin/env bash
set -euo pipefail

APP="${1:?usage: build.sh <app-name>}"
APP_DIR="$(cd "$(dirname "$0")/$APP" && pwd)"
OUT_DIR="$(cd "$(dirname "$0")" && pwd)/build/$APP"

mkdir -p "$OUT_DIR"

pnpm exec esbuild "$APP_DIR/main.ts" \
  --bundle \
  --platform=node \
  --outfile="$OUT_DIR/index.js"

echo "built $APP -> $OUT_DIR/index.js"
