#!/usr/bin/env bash
#
# Reads `govulncheck -format json` from stdin and fails on every vulnerability
# whose vulnerable symbol is actually called (the set plain `govulncheck`
# exits 3 on), except the OSV IDs allowlisted in IGNORED below.
#
# govulncheck has no ignore flag, and the JSON format always exits 0, so CI
# pipes the JSON through this script instead:
#   go run golang.org/x/vuln/cmd/govulncheck@<version> -format json ./... | bash hack/govulncheck-filter.sh

set -o errexit
set -o nounset
set -o pipefail

# Every entry needs a reason and the condition under which it can be dropped.
IGNORED=(
  # GHSA-gfjv-gqf2-c888: the Go vulndb entry only has an open-ended
  # "introduced: 0" range, so it flags every gardener version. The upstream
  # advisory says v1.145.0+ is fixed. Drop once
  # https://github.com/golang/vulndb/issues/6666 is resolved.
  GO-2026-6568
)

input=$(cat)
if ! jq -s -e 'any(.[]; .config)' &>/dev/null <<<"$input"; then
  echo "govulncheck-filter: stdin is not govulncheck JSON output" >&2
  exit 1
fi

ignored=$(printf '%s\n' "${IGNORED[@]}" | jq -R . | jq -s -c .)
report=$(jq -s -r --argjson ignored "$ignored" '
  (map(.osv // empty | {key: .id, value: .summary}) | from_entries) as $summary
  | [.[] | .finding // empty | select(.trace[0].function != null)]
  | group_by(.osv)[]
  | .[0] as $f
  | [(if ($f.osv | IN($ignored[])) then "ignored" else "VULNERABLE" end),
     $f.osv,
     "\($f.trace[0].module)@\($f.trace[0].version)",
     "fixed in \($f.fixed_version // "N/A")",
     $summary[$f.osv]]
  | join("  ")
' <<<"$input")

[[ -n "$report" ]] && echo "$report"
if grep -q '^VULNERABLE' <<<"$report"; then
  echo "govulncheck-filter: called vulnerabilities found (details: https://pkg.go.dev/vuln/<ID>)" >&2
  exit 1
fi
echo "govulncheck-filter: no unignored called vulnerabilities"
