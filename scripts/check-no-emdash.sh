#!/usr/bin/env bash
# Fails when a tracked text file contains an em-dash (U+2014); the project uses a plain hyphen.
set -euo pipefail

if hits=$(git grep -nI $'—' -- . ':!*.lock' ':!*package-lock.json'); then
  echo "em-dash (U+2014) found - use a plain hyphen:"
  echo "$hits"
  exit 1
fi
