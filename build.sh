#!/bin/bash
set -e

VERSION=${VERSION:-"0.1.0"}
APP_NAME="binance-pivot-monitor"
OUTPUT_DIR="dist"

mkdir -p "$OUTPUT_DIR"

PLATFORMS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
)

echo "Building $APP_NAME v$VERSION..."

for PLATFORM in "${PLATFORMS[@]}"; do
  GOOS=${PLATFORM%/*}
  GOARCH=${PLATFORM#*/}
  
  OUTPUT_NAME="$APP_NAME-$VERSION-$GOOS-$GOARCH"
  if [ "$GOOS" = "windows" ]; then
    OUTPUT_NAME="$OUTPUT_NAME.exe"
  fi
  
  echo "  Building $GOOS/$GOARCH..."
  
  CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build \
    -ldflags="-s -w -X main.Version=$VERSION" \
    -o "$OUTPUT_DIR/$OUTPUT_NAME" \
    ./cmd/server
done

echo ""
echo "Build complete:"
ls -lh "$OUTPUT_DIR"
