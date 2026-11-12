# ADR 0011: Archive reset by rebuild in temp file and atomic rename

* Status: Accepted
* Date: 2026-11-12

## Context and Problem Statement

`fui reset` must return the archive to initialized state without leaving partial data on crashes or
errors, and it must preserve safety guarantees (pre-reset backup, explicit confirmation). A
delete-in-place reset risks half-applied state if interrupted.

## Considered Options

1. **In-place multi-table delete/truncate transaction:** direct, but fragile against non-atomic
   filesystem/process failures and harder to guarantee clean schema defaults.
2. **Drop-and-recreate tables in place:** can be transactional, but still mutates the live file
   and increases migration coupling.
3. **Build a fresh archive in a sibling temporary file, then atomically rename over the live
   archive after backup:** crash-safe replacement model.

## Decision Outcome

Chosen option: **Reset via rebuild + atomic rename**, because:
- All rebuild work happens off to the side; the original archive remains untouched until final
  rename.
- Atomic rename gives all-or-nothing replacement on supported filesystems.
- Reset remains aligned with Plan/Apply safety, with mandatory backup before replacement.

### Consequences

* **Positive:**
  - Strong crash safety: failures before rename keep the previous archive intact.
  - Reset output state is equivalent to a newly initialized archive (or keep-settings variant).
  - Backup reason `reset` is durable and excluded from normal rotation pressure.
* **Negative:**
  - Requires temporary disk space for a second archive file.
  - Implementation must carefully reopen handles after replacement.
