package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/0xballo/fui/internal/app"
)

var reportNames = analyticsReportKinds

func configureDynamicCompletions(root *cobra.Command, deps *Deps) error {
	for _, registration := range completionRegistrations(deps) {
		cmd := findCommand(root, registration.path...)
		if cmd == nil {
			continue
		}
		if registration.args != nil {
			cmd.ValidArgsFunction = registration.args
		}
		for name, complete := range registration.flags {
			if err := cmd.RegisterFlagCompletionFunc(name, complete); err != nil {
				return fmt.Errorf("register completion for %s --%s: %w", cmd.CommandPath(), name, err)
			}
		}
	}
	return nil
}

type completionRegistration struct {
	path  []string
	args  cobra.CompletionFunc
	flags map[string]cobra.CompletionFunc
}

func completionRegistrations(deps *Deps) []completionRegistration {
	completeAccount := completeAccountNames(deps)
	completeCategory := completeCategoryNames(deps)
	completeTag := completeTagNames(deps)
	completePayee := completePayeeNames(deps)
	completeSavedReport := completeSavedReportNames(deps)

	registrations := txAndAccountCompletions(
		completeAccount,
		completeCategory,
		completeTag,
		completePayee,
		completeSavedReport,
	)
	registrations = append(
		registrations,
		reportAndRuleCompletions(completeAccount, completeCategory, completeTag, completePayee, completeSavedReport)...,
	)
	return registrations
}

func txAndAccountCompletions(
	completeAccount, completeCategory, completeTag, completePayee, completeSavedReport cobra.CompletionFunc,
) []completionRegistration {
	return []completionRegistration{
		{path: []string{"add"}, flags: map[string]cobra.CompletionFunc{
			"account":  completeAccount,
			"category": completeCategory,
		}},
		{path: []string{"transfer"}, flags: map[string]cobra.CompletionFunc{
			"from": completeAccount,
			"to":   completeAccount,
		}},
		{path: []string{"account", "show"}, args: completeSingleArg(0, completeAccount)},
		{path: []string{"account", "edit"}, args: completeSingleArg(0, completeAccount)},
		{path: []string{"account", "archive"}, args: completeSingleArg(0, completeAccount)},
		{path: []string{"category", "rename"}, args: completeSingleArg(0, completeCategory)},
		{path: []string{"category", "merge"}, args: completeSingleArg(0, completeCategory)},
		{path: []string{"category", "rm"}, args: completeSingleArg(0, completeCategory), flags: map[string]cobra.CompletionFunc{
			"reassign-to": completeCategory,
		}},
		{path: []string{"tx", "edit"}, flags: map[string]cobra.CompletionFunc{
			"category": completeCategory,
		}},
		{path: []string{"tx", "list"}, flags: map[string]cobra.CompletionFunc{
			"account":  completeAccount,
			"category": completeCategory,
			"tag":      completeTag,
			"payee":    completePayee,
		}},
		{path: []string{"tx", "search"}, flags: map[string]cobra.CompletionFunc{
			"account":  completeAccount,
			"category": completeCategory,
			"tag":      completeTag,
			"payee":    completePayee,
		}},
		{path: []string{"tx", "tag"}, args: completeTagArg(1, completeTag)},
		{path: []string{"tx", "untag"}, args: completeTagArg(1, completeTag)},
		{path: []string{"export"}, flags: map[string]cobra.CompletionFunc{
			"account":  completeAccount,
			"category": completeCategory,
			"tag":      completeTag,
			"payee":    completePayee,
		}},
		{path: []string{"import"}, flags: map[string]cobra.CompletionFunc{
			"account": completeAccount,
		}},
		{path: []string{"report"}, args: completeSingleArg(0, completeStaticValues(reportNames)), flags: map[string]cobra.CompletionFunc{
			"account": completeAccount,
		}},
		{path: []string{"report", "run"}, args: completeSingleArg(0, completeMergedValues(
			completeStaticValues([]string{"categories", "cashflow"}),
			completeSavedReport,
		))},
		{path: []string{"kpi", "runway"}, flags: map[string]cobra.CompletionFunc{
			"account": completeAccount,
		}},
	}
}

func reportAndRuleCompletions(
	completeAccount, completeCategory, completeTag, completePayee, completeSavedReport cobra.CompletionFunc,
) []completionRegistration {
	return []completionRegistration{
		{path: []string{"report", "save"}, args: completeReportSaveArgs(completeSavedKinds())},
		{path: []string{"report", "rm"}, args: completeSingleArg(0, completeSavedReport)},
		{path: []string{"rule", "add"}, flags: map[string]cobra.CompletionFunc{
			"account":  completeAccount,
			"category": completeCategory,
			"payee":    completePayee,
			"tag":      completeTag,
		}},
		{path: []string{"rule", "edit"}, flags: map[string]cobra.CompletionFunc{
			"account":  completeAccount,
			"category": completeCategory,
			"payee":    completePayee,
			"tag":      completeTag,
		}},
		{path: []string{"rule", "test"}, flags: map[string]cobra.CompletionFunc{
			"account": completeAccount,
		}},
	}
}

func completeReportSaveArgs(kinds []string) cobra.CompletionFunc {
	return func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 1 {
			return filterByPrefix(kinds, toComplete), cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func completeSavedKinds() []string {
	return reportKindLabels()
}

func completeSavedReportNames(deps *Deps) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if deps == nil || deps.Reporting.Analytics == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		rows, err := deps.Reporting.Analytics.List(context.Background())
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names := make([]string, 0, len(rows))
		for _, row := range rows {
			names = append(names, row.Name)
		}
		return filterByPrefix(names, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

func completeSingleArg(index int, source cobra.CompletionFunc) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != index {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return source(cmd, args, toComplete)
	}
}

func completeTagArg(index int, source cobra.CompletionFunc) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) < index {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return source(cmd, args, toComplete)
	}
}

func completeAccountNames(deps *Deps) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if deps == nil || deps.Service == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		rows, err := deps.Service.ListAccounts(context.Background(), app.ListAccountsRequest{IncludeArchived: true})
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names := make([]string, 0, len(rows))
		for _, row := range rows {
			names = append(names, row.Account.Name)
		}
		return filterByPrefix(names, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

func completeCategoryNames(deps *Deps) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if deps == nil || deps.Service == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		rows, err := deps.Service.ListCategories(context.Background())
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names := make([]string, 0, len(rows))
		for _, row := range rows {
			names = append(names, row.Name)
		}
		return filterByPrefix(names, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

func completeTagNames(deps *Deps) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if deps == nil || deps.Service == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		rows, err := deps.Service.ListTags(context.Background())
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names := make([]string, 0, len(rows))
		for _, row := range rows {
			names = append(names, row.Name)
		}
		return filterByPrefix(names, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

func completePayeeNames(deps *Deps) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if deps == nil || deps.Service == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		rows, err := deps.Service.ListPayees(context.Background())
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names := make([]string, 0, len(rows))
		for _, row := range rows {
			names = append(names, row.Name)
		}
		return filterByPrefix(names, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

func completeStaticValues(values []string) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return filterByPrefix(values, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

func completeMergedValues(funcs ...cobra.CompletionFunc) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		seen := make(map[string]struct{})
		out := make([]string, 0)
		for _, complete := range funcs {
			values, _ := complete(cmd, args, toComplete)
			for _, value := range values {
				if _, ok := seen[value]; ok {
					continue
				}
				seen[value] = struct{}{}
				out = append(out, value)
			}
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

func filterByPrefix(values []string, prefix string) []string {
	if prefix == "" {
		return values
	}
	foldedPrefix := strings.ToLower(prefix)
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.HasPrefix(strings.ToLower(value), foldedPrefix) {
			out = append(out, value)
		}
	}
	return out
}

func findCommand(root *cobra.Command, path ...string) *cobra.Command {
	current := root
	for _, part := range path {
		next := findChild(current, part)
		if next == nil {
			return nil
		}
		current = next
	}
	return current
}

func findChild(parent *cobra.Command, name string) *cobra.Command {
	for _, cmd := range parent.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}
