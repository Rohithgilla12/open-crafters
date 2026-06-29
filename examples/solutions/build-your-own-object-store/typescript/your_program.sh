#!/bin/sh
cd "$(dirname "$0")"
exec bun run main.ts "$@"
