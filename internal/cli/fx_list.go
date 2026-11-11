package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
)

// newFXListCmd lists the stored manual exchange rates.
func newFXListCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List manual exchange rates",
		Example: "  fui fx list --currency USD",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rt, err := runtimeOf(cmd, deps)
			if err != nil {
				return err
			}
			currency, _ := cmd.Flags().GetString("currency")
			rates, err := deps.Service.ListFXRates(cmd.Context(), currency)
			if err != nil {
				return err
			}
			return render.WriteFXRateList(deps.Stdout, rt.format, rates)
		},
	}
	cmd.Flags().String("currency", "", "filter by currency (default: every currency)")
	return cmd
}
