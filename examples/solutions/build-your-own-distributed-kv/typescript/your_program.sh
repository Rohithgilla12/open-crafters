#!/bin/sh
exec bun run "$(dirname "$0")/main.ts" "$@"
