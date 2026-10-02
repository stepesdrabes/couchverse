#!/usr/bin/env bash
# Fails when a tracked text file contains an em-dash (U+2014); the project uses a plain hyphen.
set -euo pipefail

# the UTF-8 bytes of U+2014, spelled out so this script never contains the character
emdash=$(printf '\342\200\224')
if hits=$(git grep -nIF "$emdash" -- . ':!*package-lock.json'); then
  echo "em-dash (U+2014) found - use a plain hyphen:"
  echo "$hits"
  exit 1
fi
