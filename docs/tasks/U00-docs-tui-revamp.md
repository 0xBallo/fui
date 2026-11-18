# U00 — Docs: spec update + ADR 0019
Branch `docs/u00-tui-revamp`. Owned: `docs/specs/funzionali.md`, `docs/specs/tecniche.md`, `docs/adr/0019-tui-libraries.md`, `docs/adr/README.md`.

Protocol: `docs/tasks/_PROTOCOL.md` (binding, v0.3 addendum). Plan: `docs/PLAN-tui-revamp.md` §2 (decisions are binding). Effort: low. Specs are Italian: write the spec text in Italian, the ADR in the language of the existing ADRs.

Read only: `docs/PLAN-tui-revamp.md`, F "Requisiti funzionali — TUI", T "Stack e dipendenze" and "TUI", one existing ADR (e.g. `docs/adr/0017*`) for the format.

Do:
1. F, TUI table: rewrite the Dashboard, Transazioni, Conti, Budget, Report rows per plan §1 (content + actions). Add a short "Layout" paragraph: banner, tabs, body, status, one-line help bar with `…`, `?` expands it (Global / Schermata).
2. F, Scorciatoie globali: `?` = espande la barra dei comandi; add the dashboard keys `p`, `h/l`, `t` and `i` (spiegazione) on Budget/Report.
3. T, "Stack e dipendenze" table: add glamour, ntcharts, bubble-datepicker with their purpose; note "Bubble Tea v1; la migrazione a v2 è fuori perimetro" and close the matching open point in "Decisioni tecniche e punti aperti".
4. T, token table: add the U01 tokens (`Caution`, `Info`, `IncomeSoft`, `ExpenseSoft`, `Chip`).
5. ADR 0019 (short): context (request), decision = plan §2 points 1–3, consequences (go.sum growth, `net/http` graph test stays the gate).
6. Version history line in F/T "Cronologia" as for previous changes.

No code. Verify: markdown renders, links resolve. Commit `docs: specify the tui revamp and adr 0019`.
