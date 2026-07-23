#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")"

APP="Gemini Free"
DMG="GeminiFree.dmg"

# 需要 create-dmg：brew install create-dmg
command -v create-dmg >/dev/null || { echo "请先安装：brew install create-dmg"; exit 1; }

# 确保 .app 已构建
[ -d "$APP.app" ] || ./build.sh

rm -f "$DMG"
STAGE="$(mktemp -d)"
cp -R "$APP.app" "$STAGE/"

create-dmg \
  --volname "$APP" \
  --background "assets/dmg-bg.png" \
  --window-pos 200 120 \
  --window-size 640 440 \
  --icon-size 128 \
  --icon "$APP.app" 160 170 \
  --app-drop-link 480 170 \
  --no-internet-enable \
  "$DMG" "$STAGE"

rm -rf "$STAGE"
echo "完成：$(pwd)/$DMG"
