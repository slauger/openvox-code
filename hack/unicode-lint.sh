#!/usr/bin/env bash
# Scan source files for suspicious Unicode codepoints.
#
# Catches Trojan Source attacks (bidi controls), zero-width characters,
# smart quotes, non-breaking spaces, and BOMs — anything that can hide
# meaning or sneak past code review unnoticed.
set -euo pipefail

# Byte patterns of suspicious codepoints (UTF-8):
#   U+00A0 NO-BREAK SPACE                = C2 A0
#   U+2018/2019/201C/201D smart quotes   = E2 80 98/99/9C/9D
#   U+200B/200C/200D zero-width          = E2 80 8B/8C/8D
#   U+202A..U+202E bidi embed/override   = E2 80 AA..AE
#   U+2066..U+2069 bidi isolates         = E2 81 A6..A9
#   U+FEFF BOM / ZWNBSP                  = EF BB BF
pattern=$'\xC2\xA0|\xE2\x80[\x8B\x8C\x8D\x98\x99\x9C\x9D\xAA\xAB\xAC\xAD\xAE]|\xE2\x81[\xA6\xA7\xA8\xA9]|\xEF\xBB\xBF'

mapfile -t files < <(
  find . \
    -type d \( -name .git -o -name vendor -o -name node_modules -o -name bin -o -name dist \) -prune -o \
    -type f \( \
      -name '*.go' -o \
      -name '*.yaml' -o \
      -name '*.yml' -o \
      -name '*.sh' -o \
      -name '*.json' -o \
      -name '*.toml' -o \
      -name 'Makefile' -o \
      -name 'Containerfile' -o \
      -name 'Dockerfile' \
    \) -print
)

if [ ${#files[@]} -eq 0 ]; then
  exit 0
fi

if LC_ALL=C grep -nHE "$pattern" "${files[@]}"; then
  echo "::error::Found suspicious Unicode (smart quotes, zero-width chars, bidi controls, NBSP, or BOM). Replace with ASCII equivalents."
  exit 1
fi
