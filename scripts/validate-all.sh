#!/usr/bin/env bash
# Validate reference solutions (all stages) and starters (bind) for every challenge.
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ ! -x ./crafters ]]; then
  echo "→ Building crafters"
  go build -o crafters ./cmd/crafters
fi

chmod +x examples/solutions/*/*/your_program.sh 2>/dev/null || true
chmod +x challenges/*/starters/*/your_program.sh 2>/dev/null || true

challenges=(
  build-your-own-wal
  build-your-own-queue
  build-your-own-log
  build-your-own-lsm
  build-your-own-mvcc
  build-your-own-temporal
  build-your-own-workflow-sdk
  build-your-own-raft
  build-your-own-scheduler
  build-your-own-rate-limiter
  build-your-own-object-store
  build-your-own-bloom-filter
  build-your-own-hash-ring
  build-your-own-distributed-lock
  build-your-own-id-generator
  build-your-own-distributed-cache
  build-your-own-url-shortener
  build-your-own-job-platform
  build-your-own-harness
)
langs=(python go typescript)

fail=0

echo "→ go test ./..."
go test ./...

echo "→ Grading reference solutions (19 × 3 languages, all stages)"
for ch in "${challenges[@]}"; do
  for lang in "${langs[@]}"; do
    prog="examples/solutions/${ch}/${lang}/your_program.sh"
    echo "  $ch / $lang"
    if ! ./crafters grade --challenge "$ch" --all --program "$prog"; then
      echo "FAIL reference: $ch $lang" >&2
      fail=$((fail + 1))
    fi
  done
done

echo "→ Grading starters (bind only)"
for ch in "${challenges[@]}"; do
  for lang in "${langs[@]}"; do
    prog="challenges/${ch}/starters/${lang}/your_program.sh"
    if ! ./crafters grade --challenge "$ch" --stage bind --program "$prog" >/dev/null; then
      echo "FAIL starter bind: $ch $lang" >&2
      fail=$((fail + 1))
    fi
  done
done

if [[ "$fail" -gt 0 ]]; then
  echo "✗ $fail validation(s) failed" >&2
  exit 1
fi

echo "✓ All reference solutions and starters validated"
