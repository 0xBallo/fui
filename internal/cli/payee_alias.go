package cli

import (
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/cli/render"
)

// newPayeeAliasCmd adds one or more aliases to an existing payee, keeping any it already has.
func newPayeeAliasCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "alias NAME ALIAS...",
		Short:   "Add aliases to a payee",
		Args:    minArgs(2),
		Example: `  fui payee alias Amazon "AMZN Mktp" "Amazon.it"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPayeeAlias(cmd, deps, args[0], args[1:])
		},
	}
}

func runPayeeAlias(cmd *cobra.Command, deps *Deps, name string, add []string) error {
	rt, err := runtimeOf(cmd, deps)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	payees, err := deps.Service.ListPayees(ctx)
	if err != nil {
		return err
	}
	p, err := resolvePayee(payees, name)
	if err != nil {
		return err
	}
	cur := p
	cur.Aliases = append([]string{}, p.Aliases...)
	for _, a := range add {
		if !cur.MatchesAlias(a) {
			cur.Aliases = append(cur.Aliases, a)
		}
	}
	plan, err := deps.Service.PlanRenamePayee(ctx, app.RenamePayeeRequest{
		Payee: name, NewName: p.Name, NewAliases: cur.Aliases,
	})
	if err != nil {
		return err
	}
	return writeChange(cmd, deps, rt, plan.Plan, func() error {
		out, err := deps.Service.ApplyRenamePayee(ctx, plan)
		if err != nil {
			return err
		}
		return render.WritePayee(deps.Stdout, rt.format, out)
	})
}
