#!/bin/sh
exec go run "$(dirname "$0")/main.go" "$@"
