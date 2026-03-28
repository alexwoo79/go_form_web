#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
FRONTEND_DIR="$ROOT_DIR/vue-form"
EMBED_DIR="$ROOT_DIR/ui/frontend"

echo "[1/3] Building Vue frontend..."
cd "$FRONTEND_DIR"
npm run build

echo "[2/3] Syncing dist to embed directory..."
rm -rf "$EMBED_DIR"/*
cp -R "$FRONTEND_DIR"/dist/* "$EMBED_DIR"/

echo "[3/3] Building Go binary..."
cd "$ROOT_DIR"
mkdir -p bin
go build -o bin/go-web ./cmd/server

echo "Build complete: $ROOT_DIR/bin/go-web"
