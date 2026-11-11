# ADR 0007: Golden files as contract for JSON output, schema, help, and connector streams

* Status: Accepted
* Date: 2026-11-11

## Context and Problem Statement

FUI is designed to be scriptable and machine-readable: CLI commands output structured JSON, `help --json` provides self-describing introspection for AI agents, and connectors emit `fui/1` stream records. In-code assertions (such as inspecting individual struct fields in unit tests) frequently fail to catch accidental formatting changes, missing JSON keys, reordered fields, or subtle schema regressions.

## Considered Options

1. **Manual field-by-field assertions in Go tests:** Tedious to maintain and often fails to capture the exact serialized output contract.
2. **Dynamic snapshot testing libraries:** Automates updates too aggressively, masking unintentional schema mutations behind bulk approval flags.
3. **Committed golden files (`testdata/*.golden`):** Version-controlled reference files representing exact expected outputs, checked via `go-cmp` diffs in automated test suites and CI.

## Decision Outcome

Chosen option: **Committed golden files (`testdata/*.golden`)**, because:
- Golden files serve as an explicit, human-readable contract directly visible in git pull request diffs whenever output formats change.
- Automated tests compare command outputs, help specifications, and connector streams against golden files, printing clear unified diffs on discrepancies.
- CI verifies that code generation and schema outputs match checked-in golden files, preventing silent drift between implementation and documented contracts.

### Consequences

* **Positive:**
  - High confidence in backward compatibility for automated scripts and external tooling.
  - Pull requests immediately highlight any change to machine-readable interfaces.
  - Clear, reproducible test failures with readable diffs.
* **Negative:**
  - Intentional format changes require updating the corresponding golden files in the same commit.
  - Golden files must be carefully audited during code reviews to ensure synthetic, anonymized data only.
