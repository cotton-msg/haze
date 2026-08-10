#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HAZE_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$HAZE_ROOT"

echo "=== Haze Test Suite ==="

echo ""
echo "--- Backend Tests ---"
cd "$HAZE_ROOT/backend"

echo "Running go vet..."
go vet ./...

echo "Running go test..."
go test ./... -v -count=1 -timeout 120s

echo "Building all services..."
go build ./...

echo ""
echo "--- Frontend Tests ---"
cd "$HAZE_ROOT/frontend"

echo "Running type check..."
npx vue-tsc -b --noEmit

echo "Building..."
npm run build

echo ""
echo "=== All tests passed ==="
