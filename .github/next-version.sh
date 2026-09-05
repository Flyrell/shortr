#!/usr/bin/env bash
# Prints the next release version, derived from the commit subjects since the newest
# v* tag: `breaking:` (or a conventional `!` marker) raises the major, `feat` the
# minor, anything else the patch. Without GITHUB_OUTPUT it is a dry run that writes
# to stdout, which needs a checkout with full history and tags.
set -euo pipefail

: "${GITHUB_OUTPUT:=/dev/stdout}"

previous=$(git tag --list 'v[0-9]*' --sort=-v:refname | head -n 1)
subjects=$(git log --format=%s "${previous:+$previous..}HEAD")

if [ -z "$subjects" ]; then
    echo "release=false" >>"$GITHUB_OUTPUT"
    exit 0
fi

bump=patch
while IFS= read -r subject; do
    if [[ $subject =~ ^breaking(\([^\)]*\))?!?: || $subject =~ ^[a-z]+(\([^\)]*\))?!: ]]; then
        bump=major
        break
    fi
    if [[ $subject =~ ^feat(\([^\)]*\))?: ]]; then
        bump=minor
    fi
done <<<"$subjects"

IFS=. read -r major minor patch <<<"${previous#v}"
: "${major:=0}" "${minor:=0}" "${patch:=0}"

case $bump in
    major) major=$((major + 1)) minor=0 patch=0 ;;
    minor) minor=$((minor + 1)) patch=0 ;;
    patch) patch=$((patch + 1)) ;;
esac

{
    echo "release=true"
    echo "bump=$bump"
    echo "version=$major.$minor.$patch"
    echo "tag=v$major.$minor.$patch"
} >>"$GITHUB_OUTPUT"
