#!/usr/bin/env bash
set -euo pipefail
node scripts/build.mjs
node --test "tests/node/*.test.ts"

