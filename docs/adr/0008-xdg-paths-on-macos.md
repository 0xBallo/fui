# ADR 0008: Consistent XDG base directory paths on Linux and macOS

* Status: Accepted
* Date: 2026-11-11

## Context and Problem Statement

Cross-platform CLI tools frequently face a dilemma between following Linux XDG standards (`~/.config`, `~/.local/share`, `~/.local/state`) and host-specific conventions on macOS (`~/Library/Application Support`, `~/Library/Preferences`, `~/Library/Caches`). Divergent directory layouts across operating systems complicate documentation, shell scripts, backup routines, and AI agent instructions, while fragmenting user expectations for developer-oriented terminal applications.

## Considered Options

1. **Host-native platform paths:** Use XDG on Linux, `~/Library/Application Support/fui` on macOS, and `%APPDATA%\fui` on Windows.
2. **Single legacy dotfile directory:** Store everything inside `~/.fui/`.
3. **Consistent XDG Base Directory specification across Linux and macOS:** Adhere to `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, and `XDG_STATE_HOME`, falling back to `~/.config/fui`, `~/.local/share/fui`, and `~/.local/state/fui` uniformly on both Linux and macOS.

## Decision Outcome

Chosen option: **Consistent XDG Base Directory specification across Linux and macOS**, because:
- The functional and technical specifications explicitly dictate identical standard directory paths across Linux and macOS:
  - Configuration: `~/.config/fui`
  - Data (database and backups): `~/.local/share/fui`
  - State (log files): `~/.local/state/fui`
- Developers and terminal users on macOS expect modern CLI tools to follow XDG conventions rather than hiding configuration in graphical `~/Library` hierarchies.
- Shell scripts, backup automation, and AI agent instructions remain completely portable across Linux and macOS environments without platform branching.

### Consequences

* **Positive:**
  - Identical file layouts and behavioral guarantees across Linux and macOS.
  - Simplified documentation, tests, and configuration resolution logic.
  - Respects standard `XDG_*` environment variables when set by user environments.
* **Negative:**
  - Departs from traditional macOS GUI conventions (`~/Library/Application Support`), which is acceptable and expected for a terminal-first application.
