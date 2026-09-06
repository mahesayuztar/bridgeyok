#!/usr/bin/env bash
set -euo pipefail
repositoryRoot="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ddsBuildDir="$(mktemp -d)"
trap 'rm -rf "$ddsBuildDir"' EXIT
ddsRevision=8d75755c8df81999557758c9757514edb94017bc
curl --fail --silent --show-error --location --max-time 120 "https://codeload.github.com/dds-bridge/dds/tar.gz/$ddsRevision" --output "$ddsBuildDir/source.tar.gz"
printf '%s  %s\n' 5443dd51a539747192344223ee8726b72e272176226a98d57ac9ae98560336d8 "$ddsBuildDir/source.tar.gz" | sha256sum --check --status
tar -xzf "$ddsBuildDir/source.tar.gz" -C "$ddsBuildDir"
mkdir -p "$repositoryRoot/bin"
ddsSources=()
for ddsSource in "$ddsBuildDir/dds-$ddsRevision"/src/*.cpp; do
    if [[ "$(basename "$ddsSource")" != dds.cpp ]]; then ddsSources+=("$ddsSource"); fi
done
"${CXX:-g++}" -O2 -std=c++11 -pthread -I"$ddsBuildDir/dds-$ddsRevision/include" "$repositoryRoot/tools/dds/main.cpp" "${ddsSources[@]}" -o "$repositoryRoot/bin/bridgeyok-dds"
cp "$ddsBuildDir/dds-$ddsRevision/LICENSE" "$repositoryRoot/bin/dds-LICENSE"
