# W09 — Store: saved simulations + dismissed insights (0014)

Branch `feat/w-store-sim`. Owned: store `migrations/0014_sim_insights.sql`, `queries/sim*`, `repo_sim*`, new ports. Protocol: `docs/tasks/_PROTOCOL.md`. Plan: `docs/PLAN-v0.3.md`.

Tables `saved_simulations` (name UNIQUE NOCASE, kind investment|pac|cashflow, params JSON validated, created_at) and `dismissed_insights` (key UNIQUE, kind, dismissed_at). Same method as W07; `reset --keep-settings` keeps neither (wipe); dump/restore extended.
