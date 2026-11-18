# ADR 0019: TUI libraries for the revamp, staying on Bubble Tea v1

* Status: Accepted
* Date: 2026-11-18

## Context and Problem Statement

The TUI revamp (`docs/PLAN-tui-revamp.md`) needs markdown help modals, line charts, a date picker, a truncated one-line
help bar, scrolling panels, big digits, an ASCII banner and an OSC 8 link. Bubble Tea, Bubbles and Lip Gloss v2 are
released under `charm.land/*/v2`; FUI runs on v1 with about 15k lines of TUI code. Every new dependency needs an ADR.

## Considered Options

* Migrate to the v2 ecosystem first, then add v2 libraries.
* Stay on v1 and add only the libraries that the existing stack does not cover, at their v1-compatible versions.

## Decision Outcome

Stay on v1. Add `github.com/charmbracelet/glamour` (v0.x line, Lip Gloss v1) for markdown modals,
`github.com/NimbleMarkets/ntcharts` (Bubble Tea v1 line) for Reports line/time-series charts and
`github.com/ethanefung/bubble-datepicker` (Bubble Tea v1) for the transaction date filters. Everything else uses what is
already there: `bubbles/help`, `bubbles/viewport`, `bubbles/progress`, `termenv.Hyperlink`, the house bar chart and
modal, and small hand-drawn block fonts for big digits and the banner. A library is dropped (and replaced by an
in-house component) if it brings `net/http`/`net/rpc` into `./cmd/fui`, needs Bubble Tea v2, or cannot be themed with
the semantic tokens.

### Consequences

* **Positive:** the revamp ships without a framework migration; each new library has one owner task and one purpose.
* **Negative:** larger `go.sum` (glamour brings goldmark and chroma); a later v2 migration must also move these three
  libraries to their v2 lines.
