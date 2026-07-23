#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")"

APP="Gemini Free"
BUNDLE="$APP.app"
BINDIR="$BUNDLE/Contents/MacOS"

rm -rf "$BUNDLE"
mkdir -p "$BINDIR"

echo "编译中…（arm64 + x86_64 通用二进制）"
FRAMEWORKS="-framework AppKit -framework Network -framework CryptoKit -framework ServiceManagement"
swiftc -O -o "/tmp/$APP.arm64"  Sources/*.swift -target arm64-apple-macos13  $FRAMEWORKS
swiftc -O -o "/tmp/$APP.x86_64" Sources/*.swift -target x86_64-apple-macos13 $FRAMEWORKS
lipo -create "/tmp/$APP.arm64" "/tmp/$APP.x86_64" -output "$BINDIR/$APP"
rm -f "/tmp/$APP.arm64" "/tmp/$APP.x86_64"

# 从 logo.png 生成 app 图标（macOS 自带 sips/iconutil）
mkdir -p "$BUNDLE/Contents/Resources"
ICONSET="$(mktemp -d)/AppIcon.iconset"
mkdir -p "$ICONSET"
for s in 16 32 128 256 512; do
  sips -z $s $s          logo.png --out "$ICONSET/icon_${s}x${s}.png"    >/dev/null
  sips -z $((s*2)) $((s*2)) logo.png --out "$ICONSET/icon_${s}x${s}@2x.png" >/dev/null
done
iconutil -c icns "$ICONSET" -o "$BUNDLE/Contents/Resources/AppIcon.icns"
rm -rf "$(dirname "$ICONSET")"

cat > "$BUNDLE/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>$APP</string>
  <key>CFBundleDisplayName</key><string>Gemini Free</string>
  <key>CFBundleIdentifier</key><string>com.geminirelay.app</string>
  <key>CFBundleVersion</key><string>1.1</string>
  <key>CFBundleShortVersionString</key><string>1.1</string>
  <key>CFBundleExecutable</key><string>$APP</string>
  <key>CFBundleIconFile</key><string>AppIcon</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>LSMinimumSystemVersion</key><string>13.0</string>
  <key>LSUIElement</key><true/>
</dict>
</plist>
PLIST

codesign --force --deep --sign - "$BUNDLE" 2>/dev/null || true

# 重新注册，强制 Finder 刷新图标缓存（macOS 按 bundle id 缓存，否则显示旧图标）
LSREG=/System/Library/Frameworks/CoreServices.framework/Versions/A/Frameworks/LaunchServices.framework/Versions/A/Support/lsregister
[ -x "$LSREG" ] && "$LSREG" -f "$PWD/$BUNDLE" 2>/dev/null || true

echo "完成：$(pwd)/$BUNDLE"
