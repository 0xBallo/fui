package cli

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/importing"
)

// createOption is the huh selection value meaning "create a new account for this ref".
const createOption = "\x00create"

// resolveUnmapped asks, for each unmapped connector ref of plan, whether to bind it to an
// existing account or create a new one, then re-plans the import with the chosen mappings. The
// caller (runImport) only calls this on a terminal outside --no-input and agent mode.
func resolveUnmapped(
	cmd *cobra.Command, deps *Deps, svc *importing.Service, req importing.PlanRequest, plan importing.ImportPlan,
) (importing.ImportPlan, error) {
	ctx := cmd.Context()
	accts, err := deps.Service.ListAccounts(ctx, app.ListAccountsRequest{})
	if err != nil {
		return importing.ImportPlan{}, fmt.Errorf("list accounts for unmapped refs: %w", err)
	}
	options := make([]huh.Option[string], 0, len(accts)+1)
	options = append(options, huh.NewOption("Create a new account", createOption))
	for _, a := range accts {
		options = append(options, huh.NewOption(a.Account.Name, a.Account.Name))
	}
	groups := make([]*huh.Group, len(plan.Unmapped))
	choices := make([]string, len(plan.Unmapped))
	for i, u := range plan.Unmapped {
		groups[i] = huh.NewGroup(
			huh.NewSelect[string]().
				Title(fmt.Sprintf("Account for %q (%s)", u.Name, u.Currency)).
				Options(options...).Value(&choices[i]),
		)
	}
	form := huh.NewForm(groups...).WithAccessible(true).WithInput(deps.Stdin).WithOutput(deps.Stdout)
	if err := form.Run(); err != nil {
		return importing.ImportPlan{}, fmt.Errorf("resolve unmapped accounts: %w", err)
	}
	if req.Mappings == nil {
		req.Mappings = map[string]string{}
	}
	for i, u := range plan.Unmapped {
		if choices[i] == createOption {
			req.CreateAccounts = true
			continue
		}
		req.Mappings[u.Ref] = choices[i]
	}
	return svc.PlanImport(ctx, req)
}
