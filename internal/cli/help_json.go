package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/0xballo/fui/internal/cli/render"
)

// outputSchemas names the JSON shape each command's successful output follows, keyed by full
// command path (e.g. "fui account list"). Commands without a JSON result (parent groups, backup,
// restore, config) are simply absent.
var outputSchemas = map[string]string{
	"fui init":            "init_result",
	"fui module list":     "module_state[]",
	"fui module enable":   "module_set",
	"fui module disable":  "module_set",
	"fui add":             "transaction",
	"fui tx list":         "tx_page",
	"fui tx search":       "tx_page",
	"fui tx show":         "transaction",
	"fui tx edit":         "transaction",
	"fui tx rm":           "transaction",
	"fui transfer":        "transaction[]",
	"fui account add":     "account",
	"fui account list":    "account[]",
	"fui account show":    "account",
	"fui account edit":    "account",
	"fui account archive": "account",
	"fui category add":    "category",
	"fui category list":   "category[]",
	"fui category rename": "category",
	"fui category merge":  "merge_result",
	"fui category rm":     "merge_result",
	"fui fx set":          "fx_rate",
	"fui fx list":         "fx_rate[]",
	"fui backup":          "backup_result",
	"fui restore":         "restore_result",
	"fui config get":      "config_value",
	"fui report run":      "spending_report|cashflow_report",
	"fui kpi runway":      "runway",
	"fui kpi networth":    "net_worth",
	"fui import":          "import_plan|import_result",
	"fui import undo":     "import_undo_result",
	"fui import list":     "import_batch[]",
	"fui export":          "export_result",
	"fui connector list":  "connector[]",
	"fui connector show":  "connector_detail",
	"fui connector test":  "test_report",
	"fui skill install":   "skill_install_result",
}

// helpFlag describes one flag of a command in "help --json" output.
type helpFlag struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Default   string `json:"default,omitempty"`
	Usage     string `json:"usage,omitempty"`
}

// helpCommand describes one command and its subcommands in "help --json" output.
type helpCommand struct {
	Path     string        `json:"path"`
	Short    string        `json:"short,omitempty"`
	Example  string        `json:"example,omitempty"`
	Schema   string        `json:"output_schema,omitempty"`
	Flags    []helpFlag    `json:"flags,omitempty"`
	Commands []helpCommand `json:"commands,omitempty"`
}

// describeCommand walks cmd's own subtree into the help --json shape; it recurses before
// collecting flags so children are listed in declaration order.
func describeCommand(cmd *cobra.Command) helpCommand {
	desc := helpCommand{
		Path:    cmd.CommandPath(),
		Short:   cmd.Short,
		Example: cmd.Example,
		Schema:  outputSchemas[cmd.CommandPath()],
	}
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		desc.Flags = append(desc.Flags, helpFlag{
			Name: f.Name, Shorthand: f.Shorthand, Default: f.DefValue, Usage: f.Usage,
		})
	})
	for _, c := range cmd.Commands() {
		if c.Hidden || c.Name() == "help" {
			continue
		}
		desc.Commands = append(desc.Commands, describeCommand(c))
	}
	return desc
}

// newHelpJSONCmd replaces cobra's default help command, adding --json (inherited from the root
// persistent flag) to dump the full command tree instead of printing plain usage text.
func newHelpJSONCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:                "help [command]",
		Short:              "Show help, or the full command tree with --json",
		DisableFlagParsing: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			target := cmd.Root()
			if len(args) > 0 {
				found, _, err := cmd.Root().Find(args)
				if err != nil {
					return newUsageError(fmt.Errorf("find command: %w", err))
				}
				target = found
			}
			if rt.format != render.JSON {
				return target.Help()
			}
			return render.WriteJSON(deps.Stdout, describeCommand(target))
		},
	}
}
