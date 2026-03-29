#!/usr/bin/env bash
set -euo pipefail

ALL_APPS="example echo web-echo web-echo-v2 leetcode sandbox-inspector tg-bot"

if [ "${1:-}" = "all" ]; then
  for app in $ALL_APPS; do
    bash "$(realpath "$0")" "$app"
  done
  exit 0
fi

APP="${1:?usage: build.sh <app-name|all>}"
APP_DIR="$(cd "$(dirname "$0")/$APP" && pwd)"
OUT_DIR="$(cd "$(dirname "$0")" && pwd)/build/$APP"

mkdir -p "$OUT_DIR"

pnpm exec esbuild "$APP_DIR/main.ts" \
  --bundle \
  --platform=node \
  --outfile="$OUT_DIR/index.js"

if [ -d "$APP_DIR/public" ]; then
  rm -rf "$OUT_DIR/public"
  cp -r "$APP_DIR/public" "$OUT_DIR/public"
fi

echo "built $APP -> $OUT_DIR/index.js"
