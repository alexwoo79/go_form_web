#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_NAME="go-web"
VERSION="${1:-$(date +%Y%m%d-%H%M%S)}"
DIST_ROOT="$ROOT_DIR/dist"
HOST_TARGET="$(go env GOOS)/$(go env GOARCH)"
WINDOWS_TARGET="windows/amd64"
TARGETS="${TARGETS:-$HOST_TARGET,$WINDOWS_TARGET}"

copy_runtime_files() {
  local release_dir="$1"

  if [[ -f "$ROOT_DIR/config.yaml" ]]; then
    cp "$ROOT_DIR/config.yaml" "$release_dir/config.yaml"
  elif [[ -f "$ROOT_DIR/config.example.yaml" ]]; then
    cp "$ROOT_DIR/config.example.yaml" "$release_dir/config.yaml"
  else
    echo "未找到 config.yaml 或 config.example.yaml" >&2
    exit 1
  fi

  if [[ -f "$ROOT_DIR/README.md" ]]; then
    cp "$ROOT_DIR/README.md" "$release_dir/README.md"
  fi
}

build_target() {
  local goos="$1"
  local goarch="$2"
  local release_dir="$DIST_ROOT/${APP_NAME}_${VERSION}_${goos}_${goarch}"
  local binary_name="$APP_NAME"

  rm -rf "$release_dir"
  mkdir -p "$release_dir/data"

  if [[ "$goos" == "windows" ]]; then
    binary_name="${APP_NAME}.exe"
  fi

  echo "[编译] ${goos}/${goarch}"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$release_dir/$binary_name" ./cmd/server

  copy_runtime_files "$release_dir"

  if [[ "$goos" == "windows" ]]; then
    cat > "$release_dir/run.bat" <<'EOF'
@echo off
setlocal enabledelayedexpansion
set "DIR=%~dp0"
if not exist "%DIR%data" mkdir "%DIR%data"
"%DIR%go-web.exe" -config "%DIR%config.yaml"
EOF
  else
    cat > "$release_dir/run.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
mkdir -p "$DIR/data"
exec "$DIR/go-web" -config "$DIR/config.yaml"
EOF
    chmod +x "$release_dir/run.sh"
    chmod +x "$release_dir/$binary_name"
  fi

  if [[ "$goos" == "windows" ]]; then
    local archive_path="${release_dir}.zip"
    rm -f "$archive_path"
    (
      cd "$DIST_ROOT"
      zip -r "$(basename "$archive_path")" "$(basename "$release_dir")" >/dev/null
    )
    echo "产物目录: $release_dir"
    echo "压缩包: $archive_path"
  else
    local archive_path="${release_dir}.tar.gz"
    rm -f "$archive_path"
    tar -czf "$archive_path" -C "$DIST_ROOT" "$(basename "$release_dir")"
    echo "产物目录: $release_dir"
    echo "压缩包: $archive_path"
  fi
}

mkdir -p "$DIST_ROOT"

echo "[1/3] 运行测试..."
go test ./...

echo "[2/3] 生成发行包..."
IFS=',' read -r -a target_array <<< "$TARGETS"
for target in "${target_array[@]}"; do
  goos="${target%%/*}"
  goarch="${target##*/}"
  if [[ -z "$goos" || -z "$goarch" || "$goos" == "$goarch" ]]; then
    echo "无效 TARGETS 项: $target（格式应为 os/arch）" >&2
    exit 1
  fi
  build_target "$goos" "$goarch"
done

echo "[3/3] 完成"
echo "已生成目标: $TARGETS"
