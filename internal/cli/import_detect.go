package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app/importing"
	"github.com/0xballo/fui/internal/cli/render"
)

// newImportDetectCmd suggests a CSV import profile for a file, optionally saving it.
func newImportDetectCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "detect FILE",
		Short: "Suggest a CSV import profile for a file",
		Args:  exactArgs(1),
		Example: "  fui import detect statement.csv\n" +
			"  fui import detect statement.csv --save mybank",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImportDetect(cmd, deps, args[0])
		},
	}
	cmd.Flags().String("save", "", "store the suggestion as import.profiles.NAME")
	return cmd
}

func runImportDetect(cmd *cobra.Command, deps *Deps, file string) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	svc, err := requireImporting(deps)
	if err != nil {
		return err
	}
	sug, err := svc.DetectCSV(cmd.Context(), file)
	if err != nil {
		return err
	}
	name, _ := cmd.Flags().GetString("save")
	if name != "" && !rt.dryRun {
		if rt.agent && !rt.yes {
			return confirmationRequired("FUI_AGENT=1 requires --yes to write")
		}
		if err := saveProfile(deps, name, sug); err != nil {
			return err
		}
	} else {
		name = ""
	}
	return render.WriteCSVSuggestion(deps.Stdout, rt.format, sug, name)
}

// saveProfile stores the suggested profile through the same validated path as `config set`.
func saveProfile(deps *Deps, name string, sug importing.CSVSuggestion) error {
	p := sug.Profile
	fields := [][2]string{
		{"date_column", p.DateColumn},
		{"description_column", p.DescriptionColumn},
		{"amount_column", p.AmountColumn},
		{"debit_column", p.DebitColumn},
		{"credit_column", p.CreditColumn},
		{"external_id_column", p.ExternalIDColumn},
		{"delimiter", p.Delimiter},
		{"encoding", p.Encoding},
		{"date_format", p.DateFormat},
		{"decimal_separator", p.DecimalSeparator},
	}
	if p.SkipRows > 0 {
		fields = append(fields, [2]string{"skip_rows", strconv.Itoa(p.SkipRows)})
	}
	for _, f := range fields {
		if f[1] == "" {
			continue
		}
		if err := deps.Config.Set("import.profiles."+name+"."+f[0], f[1]); err != nil {
			return fmt.Errorf("save profile %q: %w", name, err)
		}
	}
	return nil
}
