#!/usr/bin/env bash
# Prints the next release tag (e.g. v0.3.0) based on the commits since the
# last tag, or nothing if those commits don't warrant a release.
#
#   scripts/next-version.sh            # decide from commit messages
#   scripts/next-version.sh minor      # force a bump: patch | minor | major
#
# Rules (Conventional Commits, https://www.conventionalcommits.org):
#   feat!: / fix!: / "BREAKING CHANGE:" footer   -> major (minor while < v1.0.0)
#   feat:                                        -> minor
#   fix:, perf:, refactor:, unprefixed messages  -> patch
#   only docs:, ci:, chore:, test:, style:, build:, merges -> no release
# A head commit containing "[skip release]" never releases (unless forced).
set -euo pipefail

force="${1:-auto}"
case "$force" in auto | patch | minor | major) ;; *)
    echo "usage: $0 [auto|patch|minor|major]" >&2
    exit 2
    ;;
esac

last=$(git describe --tags --abbrev=0 --match 'v[0-9]*.[0-9]*.[0-9]*' 2>/dev/null || true)
if [[ -z "$last" ]]; then
    range=HEAD
    current="0.0.0"
else
    range="$last..HEAD"
    current="${last#v}"
fi

subjects=$(git log --format=%s "$range")
bodies=$(git log --format=%B "$range")

if [[ -z "$subjects" && "$force" == auto ]]; then
    exit 0 # nothing new since the last tag
fi

bump=none
if [[ "$force" != auto ]]; then
    bump="$force"
elif git log -1 --format=%B | grep -qF '[skip release]'; then
    bump=none
elif grep -qE '^[a-z]+(\([^)]*\))?!:' <<<"$subjects" || grep -qE '^BREAKING[ -]CHANGE:' <<<"$bodies"; then
    bump=major
elif grep -qE '^feat(\([^)]*\))?:' <<<"$subjects"; then
    bump=minor
elif grep -qvE '^((docs|ci|chore|test|style|build)(\([^)]*\))?:|Merge )' <<<"$subjects"; then
    bump=patch
fi

[[ "$bump" == none ]] && exit 0

# The first release is v0.1.0 whatever the commits say.
if [[ -z "$last" ]]; then
    echo "v0.1.0"
    exit 0
fi

IFS=. read -r major minor patch <<<"$current"
case "$bump" in
major)
    if ((major == 0)); then # pre-1.0: breaking changes bump the minor version
        minor=$((minor + 1)) patch=0
    else
        major=$((major + 1)) minor=0 patch=0
    fi
    ;;
minor) minor=$((minor + 1)) patch=0 ;;
patch) patch=$((patch + 1)) ;;
esac
echo "v${major}.${minor}.${patch}"
