package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/cli/render"
)

// newInitCmd creates the default categories FUI needs before the first transaction; running it
// again is harmless. --currency only sets the base currency in config.toml on success (it is not
// part of the plan, so --dry-run never changes it).
func newInitCmd(deps *Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Create the default categories",
		Example: "  fui init --currency EUR",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInit(cmd, deps)
		},
	}
	cmd.Flags().String("currency", "", "set the base currency (ISO 4217), e.g. EUR")
	return cmd
}

func runInit(cmd *cobra.Command, deps *Deps) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	plan, err := deps.Service.PlanInit(ctx)
	if err != nil {
		return err
	}
	currency, _ := cmd.Flags().GetString("currency")
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		res, err := deps.Service.ApplyInit(ctx, plan)
		if err != nil {
			return err
		}
		if currency != "" {
			if err := deps.Config.Set("currency", currency); err != nil {
				return err
			}
		}
		if rt.format == render.JSON {
			return render.WriteJSON(deps.Stdout, struct {
				Created int `json:"created"`
			}{res.Created})
		}
		_, err = fmt.Fprintf(deps.Stdout, "Created %d categories.\n", res.Created)
		if err != nil {
			return fmt.Errorf("write created-count note: %w", err)
		}
		return nil
	})
}
