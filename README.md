# fui

fui is a local-first personal finance tool: a CLI for scripts and agents, a TUI for everyday
use, one SQLite archive. No server, no account, no telemetry — your data stays in a single file
on your machine.

See `docs/specs/funzionali.md` for the full functional spec (use cases, scope) and
`docs/specs/tecniche.md` for the architecture and conventions.

## Install

Requires Go 1.26 or newer.

```sh
go install github.com/0xballo/fui/cmd/fui@latest
```

Or build from a checkout:

```sh
make build        # binaries in dist/
./dist/fui --version
```

## Quick start

```sh
fui init --currency EUR                                   # create the default categories
fui account add Checking --kind checking --currency EUR   # open an account
fui add -12,50 Esselunga -c Groceries -a Checking          # record a transaction
fui account list                                           # see balances
fui report run categories --month 2026-01                  # spending by category
fui                                                        # no command: launch the TUI
```

The archive lives under your XDG data directory by default (`$XDG_DATA_HOME/fui/fui.db`, or
`~/.local/share/fui/fui.db`); override it with `--db <path>` or `FUI_DB`.

## Commands

| Command | Purpose |
| --- | --- |
| `init` | Create the default categories |
| `account` | Add, list, show, edit or archive accounts |
| `add` | Add a transaction |
| `transfer` | Transfer money between two accounts |
| `tx` | List, show, edit or remove transactions |
| `category` | Add, list, rename, merge or remove categories |
| `report run` | Run a predefined report (categories, cashflow) |
| `kpi` | Compute a key performance indicator (e.g. runway) |
| `fx` | Set or list manual exchange rates |
| `backup` / `restore` | Back up or restore the archive |
| `config` | Read or change the configuration |
| `skill` | Show or install the agent skill |
| `help --json` | The full command tree as JSON, for scripts and agents |
| `completion` | Generate a shell completion script |

Every command supports `--json` (snake_case, amounts in minor units) and `--csv` (RFC 4180) for
scripting, and `--dry-run` to preview a write without applying it. Running `fui` with no
arguments starts the TUI.

## Using fui with AI agents

fui ships an agent skill describing when to use it, the full command/JSON reference and its
safety rules (`docs/specs/funzionali.md` § "Uso con agenti AI"):

```sh
fui skill show                       # print it
fui skill install                    # install into ~/.claude/skills/fui/SKILL.md
fui skill install --target ./skills  # or any other skills directory
```

With `FUI_AGENT=1` set, fui never prompts: every write needs an explicit `--yes`, and any
ambiguous input fails with `missing_field` instead of guessing.

## Development

```sh
make check   # fmt, generate (no diff), lint, test -race ./...
make bench   # non-functional-target benchmarks
```

See `AGENTS.md` for the full contributor/agent workflow and commit conventions.
