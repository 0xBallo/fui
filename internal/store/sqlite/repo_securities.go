package sqlite

import (
	"context"
	"fmt"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
	gen "github.com/0xballo/fui/internal/store/sqlite/sqlc"
)

type securityRepo struct {
	repos *repos
}

func (r securityRepo) Insert(ctx context.Context, s domain.Security) (domain.Security, error) {
	if err := r.repos.writable(); err != nil {
		return domain.Security{}, err
	}
	norm, err := domain.NewSecurity(s)
	if err != nil {
		return domain.Security{}, err
	}
	row, err := r.repos.queries.InsertSecurity(ctx, gen.InsertSecurityParams{
		Name:          norm.Name,
		Ticker:        toNullString(norm.Ticker),
		Isin:          toNullString(norm.ISIN),
		Kind:          string(norm.Kind),
		Currency:      norm.Currency.Code(),
		QuantityScale: int64(norm.QuantityScale),
	})
	if err != nil {
		return domain.Security{}, mapConstraint(err)
	}
	return toSecurity(row)
}

func (r securityRepo) Update(ctx context.Context, s domain.Security) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	norm, err := domain.NewSecurity(s)
	if err != nil {
		return err
	}
	changed, err := r.repos.queries.UpdateSecurity(ctx, gen.UpdateSecurityParams{
		ID:            int64(norm.ID),
		Name:          norm.Name,
		Ticker:        toNullString(norm.Ticker),
		Isin:          toNullString(norm.ISIN),
		Kind:          string(norm.Kind),
		Currency:      norm.Currency.Code(),
		QuantityScale: int64(norm.QuantityScale),
	})
	if err != nil {
		return mapConstraint(err)
	}
	if changed == 0 {
		return fmt.Errorf("security %d: %w", norm.ID, domain.ErrNotFound)
	}
	return nil
}

func (r securityRepo) Delete(ctx context.Context, id domain.SecurityID) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	changed, err := r.repos.queries.DeleteSecurity(ctx, int64(id))
	if err != nil {
		return mapConstraint(err)
	}
	if changed == 0 {
		return fmt.Errorf("security %d: %w", id, domain.ErrNotFound)
	}
	return nil
}

func (r securityRepo) Get(ctx context.Context, id domain.SecurityID) (domain.Security, error) {
	row, err := r.repos.queries.GetSecurity(ctx, int64(id))
	if err != nil {
		return domain.Security{}, mapNotFound(mapSQLError(err), "security")
	}
	return toSecurity(row)
}

func (r securityRepo) List(ctx context.Context) ([]domain.Security, error) {
	rows, err := r.repos.queries.ListSecurities(ctx)
	if err != nil {
		return nil, mapSQLError(err)
	}
	out := make([]domain.Security, 0, len(rows))
	for _, row := range rows {
		security, convErr := toSecurity(row)
		if convErr != nil {
			return nil, convErr
		}
		out = append(out, security)
	}
	return out, nil
}

func (r securityRepo) UpsertPrice(ctx context.Context, p domain.SecurityPrice) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	if p.Security == 0 {
		return fmt.Errorf("security price security: %w", domain.ErrMissingField)
	}
	if p.Date.IsZero() {
		return fmt.Errorf("security price date: %w", domain.ErrMissingField)
	}
	if p.Price.Currency().IsZero() {
		return fmt.Errorf("security price currency: %w", domain.ErrMissingField)
	}
	if p.Price.Sign() <= 0 {
		return fmt.Errorf("security price amount: %w", domain.ErrInvalidValue)
	}
	security, err := r.Get(ctx, p.Security)
	if err != nil {
		return err
	}
	if p.Price.Currency() != security.Currency {
		return fmt.Errorf("security price currency mismatch: %w", domain.ErrCurrencyMismatch)
	}
	err = r.repos.queries.UpsertSecurityPrice(ctx, gen.UpsertSecurityPriceParams{
		SecurityID: int64(p.Security),
		Date:       p.Date.String(),
		PriceMinor: p.Price.Minor(),
		Currency:   p.Price.Currency().Code(),
	})
	if err != nil {
		return mapConstraint(err)
	}
	return nil
}

func (r securityRepo) DeletePrice(ctx context.Context, securityID domain.SecurityID, date domain.Date) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	if securityID == 0 {
		return fmt.Errorf("security price security: %w", domain.ErrMissingField)
	}
	if date.IsZero() {
		return fmt.Errorf("security price date: %w", domain.ErrMissingField)
	}
	changed, err := r.repos.queries.DeleteSecurityPrice(ctx, gen.DeleteSecurityPriceParams{
		SecurityID: int64(securityID),
		Date:       date.String(),
	})
	if err != nil {
		return mapSQLError(err)
	}
	if changed == 0 {
		return fmt.Errorf("security price %d/%s: %w", securityID, date, domain.ErrNotFound)
	}
	return nil
}

func (r securityRepo) ListPrices(ctx context.Context, securityID domain.SecurityID) ([]domain.SecurityPrice, error) {
	if securityID == 0 {
		return nil, fmt.Errorf("security price security: %w", domain.ErrMissingField)
	}
	rows, err := r.repos.queries.ListSecurityPrices(ctx, int64(securityID))
	if err != nil {
		return nil, mapSQLError(err)
	}
	out := make([]domain.SecurityPrice, 0, len(rows))
	for _, row := range rows {
		price, convErr := toSecurityPrice(row)
		if convErr != nil {
			return nil, convErr
		}
		out = append(out, price)
	}
	return out, nil
}

func (r securityRepo) PriceOnOrBefore(
	ctx context.Context, securityID domain.SecurityID, date domain.Date,
) (domain.SecurityPrice, error) {
	if securityID == 0 {
		return domain.SecurityPrice{}, fmt.Errorf("security price security: %w", domain.ErrMissingField)
	}
	if date.IsZero() {
		return domain.SecurityPrice{}, fmt.Errorf("security price date: %w", domain.ErrMissingField)
	}
	row, err := r.repos.queries.SecurityPriceOnOrBefore(ctx, gen.SecurityPriceOnOrBeforeParams{
		SecurityID: int64(securityID),
		Date:       date.String(),
	})
	if err != nil {
		return domain.SecurityPrice{}, mapNotFound(mapSQLError(err), "security price on or before")
	}
	return toSecurityPrice(row)
}

func (r securityRepo) InsertOp(ctx context.Context, op domain.SecurityOp) (domain.SecurityOp, error) {
	if err := r.repos.writable(); err != nil {
		return domain.SecurityOp{}, err
	}
	norm, err := r.normalizeSecurityOp(ctx, op)
	if err != nil {
		return domain.SecurityOp{}, err
	}
	row, err := r.repos.queries.InsertSecurityOp(ctx, toInsertSecurityOpParams(norm))
	if err != nil {
		return domain.SecurityOp{}, mapConstraint(err)
	}
	return toSecurityOp(row)
}

func (r securityRepo) UpdateOp(ctx context.Context, op domain.SecurityOp) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	norm, err := r.normalizeSecurityOp(ctx, op)
	if err != nil {
		return err
	}
	changed, err := r.repos.queries.UpdateSecurityOp(ctx, gen.UpdateSecurityOpParams{
		ID:                int64(norm.ID),
		Date:              norm.Date.String(),
		AccountID:         int64(norm.Account),
		SecurityID:        int64(norm.Security),
		Kind:              string(norm.Kind),
		QuantityUnits:     norm.Quantity.Units(),
		QuantityScale:     int64(norm.Quantity.Scale()),
		UnitPriceMinor:    norm.UnitPrice.Minor(),
		UnitPriceCurrency: norm.UnitPrice.Currency().Code(),
		FeesMinor:         norm.Fees.Minor(),
		FeesCurrency:      norm.Fees.Currency().Code(),
		CashTxID:          toNullInt64(norm.CashTx),
	})
	if err != nil {
		return mapConstraint(err)
	}
	if changed == 0 {
		return fmt.Errorf("security operation %d: %w", norm.ID, domain.ErrNotFound)
	}
	return nil
}

func (r securityRepo) DeleteOp(ctx context.Context, id domain.SecurityOpID) error {
	if err := r.repos.writable(); err != nil {
		return err
	}
	changed, err := r.repos.queries.DeleteSecurityOp(ctx, int64(id))
	if err != nil {
		return mapSQLError(err)
	}
	if changed == 0 {
		return fmt.Errorf("security operation %d: %w", id, domain.ErrNotFound)
	}
	return nil
}

func (r securityRepo) GetOp(ctx context.Context, id domain.SecurityOpID) (domain.SecurityOp, error) {
	row, err := r.repos.queries.GetSecurityOp(ctx, int64(id))
	if err != nil {
		return domain.SecurityOp{}, mapNotFound(mapSQLError(err), "security operation")
	}
	return toSecurityOp(row)
}

func (r securityRepo) ListOpsBySecurity(
	ctx context.Context, securityID domain.SecurityID, from, to domain.Date,
) ([]domain.SecurityOp, error) {
	if securityID == 0 {
		return nil, fmt.Errorf("security operation security: %w", domain.ErrMissingField)
	}
	if !from.IsZero() && !to.IsZero() && from.After(to) {
		return nil, fmt.Errorf("security operation period: %w", domain.ErrInvalidValue)
	}
	rows, err := r.repos.queries.ListSecurityOpsBySecurity(ctx, gen.ListSecurityOpsBySecurityParams{
		SecurityID: int64(securityID),
		FromDate:   dateOrEmpty(from),
		ToDate:     dateOrEmpty(to),
	})
	if err != nil {
		return nil, mapSQLError(err)
	}
	out := make([]domain.SecurityOp, 0, len(rows))
	for _, row := range rows {
		op, convErr := toSecurityOp(row)
		if convErr != nil {
			return nil, convErr
		}
		out = append(out, op)
	}
	return out, nil
}

func (r securityRepo) normalizeSecurityOp(ctx context.Context, op domain.SecurityOp) (domain.SecurityOp, error) {
	norm, err := domain.NewSecurityOp(op)
	if err != nil {
		return domain.SecurityOp{}, err
	}
	if _, err := r.repos.queries.GetAccount(ctx, int64(norm.Account)); err != nil {
		return domain.SecurityOp{}, mapNotFound(mapSQLError(err), "security operation account")
	}
	security, err := r.Get(ctx, norm.Security)
	if err != nil {
		return domain.SecurityOp{}, err
	}
	if norm.Quantity.Scale() != security.QuantityScale {
		return domain.SecurityOp{}, fmt.Errorf("security operation quantity scale mismatch: %w", domain.ErrInvalidValue)
	}
	if norm.UnitPrice.Currency() != security.Currency {
		return domain.SecurityOp{}, fmt.Errorf("security operation unit price currency mismatch: %w", domain.ErrCurrencyMismatch)
	}
	if norm.CashTx != 0 {
		if _, err := r.repos.queries.GetTransaction(ctx, int64(norm.CashTx)); err != nil {
			return domain.SecurityOp{}, mapNotFound(mapSQLError(err), "security operation cash transaction")
		}
	}
	return norm, nil
}

func toInsertSecurityOpParams(op domain.SecurityOp) gen.InsertSecurityOpParams {
	return gen.InsertSecurityOpParams{
		Date:              op.Date.String(),
		AccountID:         int64(op.Account),
		SecurityID:        int64(op.Security),
		Kind:              string(op.Kind),
		QuantityUnits:     op.Quantity.Units(),
		QuantityScale:     int64(op.Quantity.Scale()),
		UnitPriceMinor:    op.UnitPrice.Minor(),
		UnitPriceCurrency: op.UnitPrice.Currency().Code(),
		FeesMinor:         op.Fees.Minor(),
		FeesCurrency:      op.Fees.Currency().Code(),
		CashTxID:          toNullInt64(op.CashTx),
	}
}

func dateOrEmpty(d domain.Date) string {
	if d.IsZero() {
		return ""
	}
	return d.String()
}

func toSecurity(row gen.Security) (domain.Security, error) {
	kind, err := domain.ParseSecurityKind(row.Kind)
	if err != nil {
		return domain.Security{}, fmt.Errorf("parse security kind %q: %w", row.Kind, err)
	}
	cur, err := domain.ParseCurrency(row.Currency)
	if err != nil {
		return domain.Security{}, fmt.Errorf("parse security currency %q: %w", row.Currency, err)
	}
	security, err := domain.NewSecurity(domain.Security{
		ID:            domain.SecurityID(row.ID),
		Name:          row.Name,
		Ticker:        nullStringOrEmpty(row.Ticker),
		ISIN:          nullStringOrEmpty(row.Isin),
		Kind:          kind,
		Currency:      cur,
		QuantityScale: int(row.QuantityScale),
	})
	if err != nil {
		return domain.Security{}, fmt.Errorf("parse security row: %w", err)
	}
	return security, nil
}

func toSecurityPrice(row gen.SecurityPrice) (domain.SecurityPrice, error) {
	cur, err := domain.ParseCurrency(row.Currency)
	if err != nil {
		return domain.SecurityPrice{}, fmt.Errorf("parse security price currency %q: %w", row.Currency, err)
	}
	date, err := domain.ParseDate(row.Date)
	if err != nil {
		return domain.SecurityPrice{}, fmt.Errorf("parse security price date %q: %w", row.Date, err)
	}
	price, err := domain.NewMoney(row.PriceMinor, cur)
	if err != nil {
		return domain.SecurityPrice{}, fmt.Errorf("parse security price amount: %w", err)
	}
	if price.Sign() <= 0 {
		return domain.SecurityPrice{}, fmt.Errorf("parse security price amount: %w", domain.ErrInvalidValue)
	}
	return domain.SecurityPrice{
		Security: domain.SecurityID(row.SecurityID),
		Date:     date,
		Price:    price,
	}, nil
}

func toSecurityOp(row gen.SecurityOp) (domain.SecurityOp, error) {
	date, err := domain.ParseDate(row.Date)
	if err != nil {
		return domain.SecurityOp{}, fmt.Errorf("parse security operation date %q: %w", row.Date, err)
	}
	kind, err := domain.ParseSecurityOpKind(row.Kind)
	if err != nil {
		return domain.SecurityOp{}, fmt.Errorf("parse security operation kind %q: %w", row.Kind, err)
	}
	quantity, err := domain.NewQuantity(row.QuantityUnits, int(row.QuantityScale))
	if err != nil {
		return domain.SecurityOp{}, fmt.Errorf("parse security operation quantity: %w", err)
	}
	unitPriceCurrency, err := domain.ParseCurrency(row.UnitPriceCurrency)
	if err != nil {
		return domain.SecurityOp{}, fmt.Errorf("parse security operation unit price currency %q: %w", row.UnitPriceCurrency, err)
	}
	unitPrice, err := domain.NewMoney(row.UnitPriceMinor, unitPriceCurrency)
	if err != nil {
		return domain.SecurityOp{}, fmt.Errorf("parse security operation unit price: %w", err)
	}
	feesCurrency, err := domain.ParseCurrency(row.FeesCurrency)
	if err != nil {
		return domain.SecurityOp{}, fmt.Errorf("parse security operation fees currency %q: %w", row.FeesCurrency, err)
	}
	fees, err := domain.NewMoney(row.FeesMinor, feesCurrency)
	if err != nil {
		return domain.SecurityOp{}, fmt.Errorf("parse security operation fees: %w", err)
	}
	op, err := domain.NewSecurityOp(domain.SecurityOp{
		ID:        domain.SecurityOpID(row.ID),
		Date:      date,
		Account:   domain.AccountID(row.AccountID),
		Security:  domain.SecurityID(row.SecurityID),
		Kind:      kind,
		Quantity:  quantity,
		UnitPrice: unitPrice,
		Fees:      fees,
		CashTx:    domain.TransactionID(nullInt64OrZero(row.CashTxID)),
	})
	if err != nil {
		return domain.SecurityOp{}, fmt.Errorf("parse security operation row: %w", err)
	}
	return op, nil
}

var _ app.SecurityRepo = securityRepo{}
