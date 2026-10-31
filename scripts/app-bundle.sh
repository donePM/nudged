#!/bin/sh
# Usage: app-bundle.sh <binary> <version> <outdir>
# Makes <outdir>/nudged.app and <outdir>/nudged_<version>_darwin_universal.zip.
set -eu
[ $# -eq 3 ] || { echo "usage: $0 <binary> <version> <outdir>" >&2; exit 2; }
bin=$1 version=$2 out=$3
plist="$(cd "$(dirname "$0")/.." && pwd)/build/darwin/Info.plist"
app="$out/nudged.app"
zip="nudged_${version}_darwin_universal.zip"

rm -rf "$app" "$out/$zip"
mkdir -p "$app/Contents/MacOS"
cp "$bin" "$app/Contents/MacOS/nudged"
chmod 755 "$app/Contents/MacOS/nudged"
cp "$plist" "$app/Contents/Info.plist"
plutil -replace CFBundleShortVersionString -string "$version" "$app/Contents/Info.plist"
plutil -replace CFBundleVersion -string "$version" "$app/Contents/Info.plist"
(cd "$out" && ditto -c -k --keepParent nudged.app "$zip")
