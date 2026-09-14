#!/usr/bin/env bash
# MediaVault portable setup script.
# Builds the app and installs a complete, self-contained instance
# (binary + database + storage + config) at the target directory you choose —
# e.g. a mount point on an external SSD.
#
# Usage:
#   ./setup.sh /path/to/install/dir
#
# Example (external SSD mounted at /media/mohamed/MySSD):
#   ./setup.sh /media/mohamed/MySSD/mediavault

set -euo pipefail

TARGET_DIR="${1:-}"

if [ -z "$TARGET_DIR" ]; then
	echo "Usage: $0 /path/to/install/dir"
	echo ""
	echo "Example: $0 /media/\$USER/MySSD/mediavault"
	exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "==> Checking dependencies..."
for cmd in go templ ffmpeg; do
	if ! command -v "$cmd" >/dev/null 2>&1; then
		echo "ERROR: '$cmd' is required but not found in PATH." >&2
		exit 1
	fi
done
echo "    go, templ, ffmpeg all found."

echo "==> Preparing target directory: $TARGET_DIR"
mkdir -p "$TARGET_DIR/db" "$TARGET_DIR/storage"

echo "==> Building the binary..."
cd "$SCRIPT_DIR"
templ generate
CGO_ENABLED=1 go build -o "$TARGET_DIR/mediavault" ./cmd/api

echo "==> Writing configuration..."
ENV_FILE="$TARGET_DIR/.env"
if [ -f "$ENV_FILE" ]; then
	echo "    $ENV_FILE already exists — leaving it untouched."
else
	JWT_SECRET="$(head -c 64 /dev/urandom | base64 | tr -d '\n')"
	cat > "$ENV_FILE" <<EOF
PORT=8080
APP_ENV=local
BLUEPRINT_DB_URL=$TARGET_DIR/db/mediavault.db
STORAGE_PATH=$TARGET_DIR/storage
JWT_SECRET=$JWT_SECRET
EOF
	echo "    Wrote fresh $ENV_FILE with a newly generated JWT_SECRET."
fi

echo "==> Running once to apply database migrations..."
(
	cd "$TARGET_DIR"
	timeout 5 ./mediavault > /tmp/mediavault-setup.log 2>&1 || true
)
if grep -q "Starting HTTP server" /tmp/mediavault-setup.log 2>/dev/null; then
	echo "    Migrations applied, server starts correctly."
else
	echo "    WARNING: could not confirm clean startup — check /tmp/mediavault-setup.log"
fi
rm -f /tmp/mediavault-setup.log

echo ""
echo "==> Done. Installed at: $TARGET_DIR"
echo ""
echo "To run it manually:"
echo "  cd $TARGET_DIR && ./mediavault"
echo ""
echo "Then open http://localhost:8080 (or http://<this-PC's-LAN-IP>:8080 from another device)."
