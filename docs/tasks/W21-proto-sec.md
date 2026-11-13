# W21 — Proto: security ops in fui/1

Branch `feat/w-proto-sec`. Owned: `pkg/fuiproto/`, `internal/app/importing/`. Protocol: `docs/tasks/_PROTOCOL.md`. Plan: `docs/PLAN-v0.3.md`.

Like V33: additive optional record `security_op` (and `security` metadata) in fui/1 (no major bump), schema + conformance + goldens; export writes, import applies (only when `investments` active, otherwise ignored with a warning); round-trip property test.
