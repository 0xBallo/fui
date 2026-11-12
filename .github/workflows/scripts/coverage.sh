#!/usr/bin/env bash
# coverage.sh runs the whole test suite with cross-package coverage instrumentation
# (-coverpkg=./...), strips generated and test-only code from the resulting profile, and reports
# the total and internal/domain coverage percentages. See docs/specs/tecniche.md, "Regole di
# copertura", for the thresholds enforced here.
#
# Usage: coverage.sh [--enforce]
#   --enforce  exit non-zero if total coverage is below 80% or internal/domain is below 90%.
#              Without it the script only prints the measured percentages (used by `make cover`).
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

enforce=0
if [[ "${1:-}" == "--enforce" ]]; then
  enforce=1
fi

total_threshold=80
domain_threshold=90

# Packages that are generated, test-only, or test infrastructure: excluded from the coverage
# count per the technical spec.
exclude_regex='github\.com/0xballo/fui/(internal/store/sqlite/sqlc|internal/testutil|internal/tui/screens/screentest|tools)/'

raw_profile=$(mktemp)
filtered_profile=$(mktemp)
domain_profile=$(mktemp)
trap 'rm -f "$raw_profile" "$filtered_profile" "$domain_profile"' EXIT

go test -count=1 -coverprofile="$raw_profile" -coverpkg=./... ./...

head -n1 "$raw_profile" > "$filtered_profile"
tail -n +2 "$raw_profile" | grep -Ev "$exclude_regex" >> "$filtered_profile" || true

total=$(go tool cover -func="$filtered_profile" | awk '/^total:/ {gsub("%", "", $3); print $3}')

head -n1 "$filtered_profile" > "$domain_profile"
tail -n +2 "$filtered_profile" | grep 'github\.com/0xballo/fui/internal/domain/' >> "$domain_profile" || true
if [[ $(wc -l < "$domain_profile") -gt 1 ]]; then
  domain=$(go tool cover -func="$domain_profile" | awk '/^total:/ {gsub("%", "", $3); print $3}')
else
  domain="0.0"
fi

echo "coverage: total=${total}% (want >= ${total_threshold}%), internal/domain=${domain}% (want >= ${domain_threshold}%)"

if [[ "$enforce" -eq 1 ]]; then
  fail=0
  if awk -v v="$total" -v min="$total_threshold" 'BEGIN { exit !(v + 0 < min) }'; then
    echo "FAIL: total coverage ${total}% is below ${total_threshold}%"
    fail=1
  fi
  if awk -v v="$domain" -v min="$domain_threshold" 'BEGIN { exit !(v + 0 < min) }'; then
    echo "FAIL: internal/domain coverage ${domain}% is below ${domain_threshold}%"
    fail=1
  fi
  if [[ "$fail" -eq 1 ]]; then
    exit 1
  fi
fi
