// Package lists implements the TUI sub-views that manage categories, tags, rules, recurrences
// and import profiles: CRUD lists meant to be embedded as sections of the settings shell (see
// internal/tui/screens/settings). Each sub-view is its own screens.Screen so the shell can
// delegate Update/View/SetSize to whichever one is active.
package lists

import (
	"context"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/assetting"
	"github.com/0xballo/fui/internal/app/recurring"
	"github.com/0xballo/fui/internal/app/rules"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// CategoriesUseCases is the subset of app.Service the categories sub-view needs.
type CategoriesUseCases interface {
	ListCategories(ctx context.Context) ([]domain.Category, error)
	PlanAddCategory(ctx context.Context, req app.AddCategoryRequest) (app.AddCategoryPlan, error)
	ApplyAddCategory(ctx context.Context, p app.AddCategoryPlan) (domain.Category, error)
	PlanRenameCategory(ctx context.Context, req app.RenameCategoryRequest) (app.RenameCategoryPlan, error)
	ApplyRenameCategory(ctx context.Context, p app.RenameCategoryPlan) (domain.Category, error)
	PlanRemoveCategory(ctx context.Context, req app.RemoveCategoryRequest) (app.RemoveCategoryPlan, error)
	ApplyRemoveCategory(ctx context.Context, p app.RemoveCategoryPlan) (app.RemoveCategoryResult, error)
}

// TagsUseCases is the subset of app.Service the tags sub-view needs.
type TagsUseCases interface {
	ListTags(ctx context.Context) ([]domain.Tag, error)
	PlanAddTag(ctx context.Context, req app.AddTagRequest) (app.AddTagPlan, error)
	ApplyAddTag(ctx context.Context, p app.AddTagPlan) (domain.Tag, error)
	PlanRenameTag(ctx context.Context, req app.RenameTagRequest) (app.RenameTagPlan, error)
	ApplyRenameTag(ctx context.Context, p app.RenameTagPlan) (domain.Tag, error)
	PlanRemoveTag(ctx context.Context, req app.RemoveTagRequest) (app.RemoveTagPlan, error)
	ApplyRemoveTag(ctx context.Context, p app.RemoveTagPlan) error
}

// RulesUseCases is the subset of rules.Service (CRUD and test) plus the app.Service lookups the
// rules sub-view needs to resolve and suggest accounts, categories, payees and tags.
type RulesUseCases interface {
	List(ctx context.Context) ([]domain.Rule, error)
	PlanAdd(ctx context.Context, req rules.AddRequest) (rules.RulePlan, error)
	ApplyAdd(ctx context.Context, p rules.RulePlan) (domain.Rule, error)
	PlanEdit(ctx context.Context, req rules.EditRequest) (rules.RulePlan, error)
	ApplyEdit(ctx context.Context, p rules.RulePlan) error
	PlanRemove(ctx context.Context, name string) (rules.RulePlan, error)
	ApplyRemove(ctx context.Context, p rules.RulePlan) error
	Test(ctx context.Context, req rules.TestRequest) (rules.TestResult, error)
	ListAccounts(ctx context.Context) ([]domain.Account, error)
	ListCategories(ctx context.Context) ([]domain.Category, error)
	ListPayees(ctx context.Context) ([]domain.Payee, error)
	ListTags(ctx context.Context) ([]domain.Tag, error)
}

// RecurrencesUseCases is the subset of recurring.Service plus the app.Service lookups the
// recurrences sub-view needs to resolve and suggest accounts and categories.
type RecurrencesUseCases interface {
	List(ctx context.Context, enabled module.Set) ([]app.Recurrence, error)
	PlanAdd(ctx context.Context, enabled module.Set, req recurring.AddRequest) (recurring.RecurrencePlan, error)
	ApplyAdd(ctx context.Context, p recurring.RecurrencePlan) (app.Recurrence, error)
	PlanEdit(ctx context.Context, enabled module.Set, req recurring.EditRequest) (recurring.RecurrencePlan, error)
	ApplyEdit(ctx context.Context, p recurring.RecurrencePlan) error
	PlanRemove(ctx context.Context, enabled module.Set, id app.RecurrenceID) (recurring.RecurrencePlan, error)
	ApplyRemove(ctx context.Context, p recurring.RecurrencePlan) error
	PlanRun(ctx context.Context, enabled module.Set, req recurring.RunRequest) (recurring.RunPlan, error)
	ApplyRun(ctx context.Context, p recurring.RunPlan) (int, error)
	ListAccounts(ctx context.Context) ([]domain.Account, error)
	ListCategories(ctx context.Context) ([]domain.Category, error)
}

// AssetsUseCases is the subset of assetting.Service the assets sub-view needs, plus the loan
// names offered when linking an asset to a loan.
type AssetsUseCases interface {
	List(ctx context.Context, enabled module.Set, at string) ([]assetting.Item, error)
	Show(ctx context.Context, enabled module.Set, ref, at string) (assetting.View, error)
	PlanAdd(ctx context.Context, enabled module.Set, req assetting.AddRequest) (assetting.AddPlan, error)
	ApplyAdd(ctx context.Context, p assetting.AddPlan) (domain.Asset, error)
	PlanEdit(ctx context.Context, enabled module.Set, req assetting.EditRequest) (assetting.EditPlan, error)
	ApplyEdit(ctx context.Context, p assetting.EditPlan) (domain.Asset, error)
	PlanRemove(ctx context.Context, enabled module.Set, ref string) (assetting.RemovePlan, error)
	ApplyRemove(ctx context.Context, p assetting.RemovePlan) error
	PlanValue(ctx context.Context, enabled module.Set, req assetting.ValueRequest) (assetting.ValuePlan, error)
	ApplyValue(ctx context.Context, p assetting.ValuePlan) error
	PlanRemoveValuation(ctx context.Context, enabled module.Set, ref, date string) (assetting.RemoveValuationPlan, error)
	ApplyRemoveValuation(ctx context.Context, p assetting.RemoveValuationPlan) error
	PlanLink(ctx context.Context, enabled module.Set, assetRef, loanRef string) (assetting.LinkPlan, error)
	ApplyLink(ctx context.Context, p assetting.LinkPlan) error
	PlanUnlink(ctx context.Context, enabled module.Set, assetRef string) (assetting.UnlinkPlan, error)
	ApplyUnlink(ctx context.Context, p assetting.UnlinkPlan) error
	ListLoanNames(ctx context.Context, enabled module.Set) ([]string, error)
}

// Profile is the user-facing shape of a CSV import profile, independent from internal/config's
// persisted representation (internal/tui must not import internal/config: see AGENTS.md).
type Profile struct {
	Name              string
	DateColumn        string
	AmountColumn      string
	DebitColumn       string
	CreditColumn      string
	DescriptionColumn string
	ExternalIDColumn  string
	Delimiter         string
	Encoding          string
	DateFormat        string
	DecimalSeparator  string
	SkipRows          string
	TargetAccount     string
}

// ProfilesConfig is the narrow configuration port the import-profiles sub-view needs to list,
// write and delete named CSV import profiles; an adapter over *config.Config satisfies it
// without this package importing internal/config.
type ProfilesConfig interface {
	// ListProfiles returns every saved profile, ordered by name.
	ListProfiles() ([]Profile, error)
	// SaveProfile creates or replaces the named profile.
	SaveProfile(p Profile) error
	// DeleteProfile removes the named profile.
	DeleteProfile(name string) error
}
