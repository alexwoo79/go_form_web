#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
FRONTEND_DIR="$ROOT_DIR/vue-form"
EMBED_DIR="$ROOT_DIR/ui/frontend"
TARGET="${1:-native}"

usage() {
	cat <<'EOF'
Usage: ./build.sh [native|windows|all]

	native   Build current platform binary (default)
	windows  Build Windows amd64 binary
	all      Build both native and Windows binaries
EOF
}

build_native() {
	echo "[3/3] Building Go native binary..."
	cd "$ROOT_DIR"
	mkdir -p bin
	go build -o bin/go-web ./cmd/server
	echo "Build complete: $ROOT_DIR/bin/go-web"
}

build_windows() {
	echo "[3/3] Building Go Windows binary..."
	cd "$ROOT_DIR"
	mkdir -p bin
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/go-web.exe ./cmd/server
	echo "Build complete: $ROOT_DIR/bin/go-web.exe"
}

echo "[1/3] Building Vue frontend..."
cd "$FRONTEND_DIR"
npm run build

echo "[2/3] Syncing dist to embed directory..."
rm -rf "$EMBED_DIR"/*
cp -R "$FRONTEND_DIR"/dist/* "$EMBED_DIR"/

case "$TARGET" in
	native)
		build_native
		;;
	windows)
		build_windows
		;;
	all)
		build_native
		build_windows
		;;
	-h|--help|help)
		usage
		;;
	*)
		echo "Unknown target: $TARGET"
		usage
		exit 1
		;;
esac
