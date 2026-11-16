package main

import (
	"context"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/app/investing"
	"github.com/0xballo/fui/internal/app/reporting"
	"github.com/0xballo/fui/internal/app/simulating"
	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
)

// The adapters below back the v0.3 TUI features. Like the v0.2 ones they build their service per
// call, because the module gate lives in the service and Settings can change the set live.

// positionsUseCases adapts investing.Service to accounts.PositionsUseCases.
type positionsUseCases struct{ build func() *investing.Service }

func (u positionsUseCases) ListPositions(ctx context.Context, req investing.PositionsRequest) ([]investing.PositionLine, error) {
	return u.build().ListPositions(ctx, req)
}

func (u positionsUseCases) PlanSetPrice(ctx context.Context, req investing.SetPriceRequest) (investing.SetPricePlan, error) {
	return u.build().PlanSetPrice(ctx, req)
}

func (u positionsUseCases) ApplySetPrice(ctx context.Context, p investing.SetPricePlan) error {
	return u.build().ApplySetPrice(ctx, p)
}

// netWorthUseCases adapts reporting.Analytics to dashboard.NetWorthUseCases.
type netWorthUseCases struct{ analytics func() *reporting.Analytics }

func (u netWorthUseCases) NetWorthAt(ctx context.Context, at domain.Date) (reporting.NetWorthPoint, error) {
	return u.analytics().NetWorthAt(ctx, at)
}

// compareUseCases adapts reporting.Analytics to report.CompareUseCases.
type compareUseCases struct{ analytics func() *reporting.Analytics }

func (u compareUseCases) ComparePeriods(ctx context.Context, req reporting.CompareRequest) (reporting.CompareReport, error) {
	return u.analytics().ComparePeriods(ctx, req)
}

// insightUseCases adapts reporting.Insights to dashboard.InsightUseCases.
type insightUseCases struct{ build func() *reporting.Insights }

func (u insightUseCases) Run(ctx context.Context) ([]reporting.InsightView, error) {
	return u.build().Run(ctx)
}

func (u insightUseCases) Dismiss(ctx context.Context, key string) error {
	return u.build().Dismiss(ctx, key)
}

func (u insightUseCases) AcceptSubscription(ctx context.Context, key string) (reporting.AcceptResult, error) {
	return u.build().AcceptSubscription(ctx, key)
}

// simUseCases adapts simulating.Service, binding the live enabled-module set, to report.SimUseCases.
type simUseCases struct {
	svc     *simulating.Service
	modules func() module.Set
	base    domain.Currency
}

func (u simUseCases) Run(ctx context.Context, req simulating.RunRequest) (simulating.RunResult, error) {
	return u.svc.Run(ctx, u.modules(), req)
}

func (u simUseCases) PlanSave(ctx context.Context, name string, req simulating.RunRequest) (simulating.SavePlan, error) {
	return u.svc.PlanSave(ctx, u.modules(), name, req)
}

func (u simUseCases) ApplySave(ctx context.Context, p simulating.SavePlan) (app.SavedSimulation, error) {
	return u.svc.ApplySave(ctx, p)
}

func (u simUseCases) List(ctx context.Context) ([]app.SavedSimulation, error) {
	return u.svc.List(ctx, u.modules())
}

func (u simUseCases) Rerun(ctx context.Context, ref string) (simulating.RerunResult, error) {
	return u.svc.Rerun(ctx, u.modules(), ref)
}

func (u simUseCases) Compare(ctx context.Context, refs []string) (simulating.CompareResult, error) {
	return u.svc.Compare(ctx, u.modules(), refs)
}

func (u simUseCases) BaseCurrency() domain.Currency { return u.base }
