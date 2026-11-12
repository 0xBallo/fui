# T12 — CI green and coverage gate

Branch `fix/ci`. Owned paths: `.github/`, `Makefile`, `go.mod`, `go.sum`, `cmd/fui-connector-widiba/`,
`internal/connector/pipeline_test.go`, `tools/anonymize/`. Protocol: `docs/tasks/_PROTOCOL.md`.

The CI on `main` (`.github/workflows/ci.yml`) fails; see `gh run view 43213689000 --log-failed`. Fix the root
causes, do not weaken checks:

1. **lint**: `golangci/golangci-lint-action@v6` installs golangci-lint v1.64 (built with go1.24) which refuses
   `go 1.27.4`. Move to the action major that supports golangci-lint v2 and pin a v2 version built with Go ≥ 1.27
   (local reference: `golangci-lint --version` → 2.15.0). Config is already `version: "2"`.
2. **security**: govulncheck reports called vulns GO-2026-6677..6689 in `github.com/xuri/excelize/v2@v2.10.0`.
   Upgrade to the first fixed release (`go list -m -versions github.com/xuri/excelize/v2`, v2.11.1 expected),
   `go mod tidy`, re-run widiba/anonymize tests.
3. **test go1.26**: `cmd/fui-connector-widiba` `TestFetchGolden` mismatches only with Go 1.26 (passes on 1.27).
   Reproduce with `GOTOOLCHAIN=go1.26.<latest> go test ./cmd/fui-connector-widiba/` (toolchain download is
   allowed for this task), find the root cause (likely the xls/xlsx parser or float/number formatting differing
   by Go version, or the excelize upgrade itself) and make the output deterministic across 1.26 and 1.27.
4. **test perf**: `internal/connector` `TestImportTenThousandRowsUnderFiveSeconds` runs under `-race` in
   `make test` and takes 7–15 s on CI runners. The spec wants performance verified by benchmarks: skip the timing
   assertion when the race detector is on (build-tagged `race` const) and add/keep a `Benchmark...` for 10 000
   rows; keep the functional part of the test running always.
5. **coverage job**: add a `coverage` job (spec "Regole di copertura": ≥ 80% total, ≥ 90% `internal/domain`).
   Measure with `-coverpkg=./...` and exclude generated/test-only code (`internal/store/sqlite/sqlc`,
   `internal/testutil/...`, `internal/tui/screens/screentest`, `tools/...`) via a small script in
   `.github/workflows/scripts/coverage.sh` used by both `make cover` and CI. Report the measured numbers in the
   report (`VERIFY: cover=total:..,domain:..`). If a threshold is not met **do not lower it**: the job must exist
   and enforce it; put the gap in `GAPS:` (other tasks own those packages and will raise it).
6. Also run `go test -race ./...` with `TZ=Pacific/Kiritimati` locally.

Acceptance: every job of `ci.yml` would pass (coverage job may fail only on the thresholds, stated in GAPS).
