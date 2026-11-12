package render

import (
	"fmt"
	"io"

	"github.com/0xballo/fui/internal/app"
	"github.com/0xballo/fui/internal/domain"
)

// LoanView is the CLI-resolved shape of one loan: the domain loan plus the liability account and
// the debit account name, which the CLI has already looked up (Loan.DebitAccount is only an ID).
type LoanView struct {
	Loan             domain.Loan
	Account          domain.Account // liability account
	DebitAccountName string
	Events           []domain.LoanEvent
}

// LoanEventDTO is the --json shape of one loan event.
type LoanEventDTO struct {
	Kind          string `json:"kind"`
	Date          string `json:"date"`
	NewRateBP     int64  `json:"new_rate_bp,omitempty"`
	NewTermMonths int    `json:"new_term_months,omitempty"`
	Prepaid       int64  `json:"prepaid,omitempty"`
	Mode          string `json:"mode,omitempty"`
}

func loanEventDTO(e domain.LoanEvent) LoanEventDTO {
	out := LoanEventDTO{Kind: string(e.Kind), Date: e.EventDate.String()}
	if e.NewRate != nil {
		out.NewRateBP = e.NewRate.BasisPoints()
	}
	if e.NewTermMonths != nil {
		out.NewTermMonths = *e.NewTermMonths
	}
	if e.PrepaidAmount != nil {
		out.Prepaid = e.PrepaidAmount.Minor()
	}
	if e.PrepayMode != nil {
		out.Mode = string(*e.PrepayMode)
	}
	return out
}

// LoanDTO is the --json shape of one loan.
type LoanDTO struct {
	ID               int64          `json:"id"`
	Name             string         `json:"name"`
	Kind             string         `json:"kind"`
	Principal        int64          `json:"principal"`
	Currency         string         `json:"currency"`
	DisbursedOn      string         `json:"disbursed_on"`
	TermMonths       int            `json:"term_months"`
	RateType         string         `json:"rate_type"`
	DebitAccount     string         `json:"debit_account"`
	LiabilityAccount string         `json:"liability_account"`
	Events           []LoanEventDTO `json:"events,omitempty"`
}

func loanDTO(v LoanView) LoanDTO {
	out := LoanDTO{
		ID:               int64(v.Loan.ID),
		Name:             v.Loan.Name,
		Kind:             string(v.Loan.Kind),
		Principal:        v.Loan.Principal.Minor(),
		Currency:         v.Loan.Principal.Currency().Code(),
		DisbursedOn:      v.Loan.DisbursedOn.String(),
		TermMonths:       v.Loan.TermMonths,
		RateType:         string(v.Loan.RateType),
		DebitAccount:     v.DebitAccountName,
		LiabilityAccount: v.Account.Name,
	}
	for _, e := range v.Events {
		out.Events = append(out.Events, loanEventDTO(e))
	}
	return out
}

func loanRows(v LoanView, loc domain.Locale) Rows {
	return Rows{
		Header: []string{"FIELD", "VALUE"},
		Body: [][]string{
			{"name", v.Loan.Name},
			{"kind", string(v.Loan.Kind)},
			{"principal", FormatMoney(v.Loan.Principal, loc)},
			{"disbursed_on", FormatDate(v.Loan.DisbursedOn, loc)},
			{"term_months", fmt.Sprint(v.Loan.TermMonths)},
			{"rate_type", string(v.Loan.RateType)},
			{"debit_account", v.DebitAccountName},
			{"liability_account", v.Account.Name},
		},
	}
}

func loanEventRows(events []domain.LoanEvent, loc domain.Locale) Rows {
	t := Rows{Header: []string{"KIND", "DATE", "RATE", "TERM_MONTHS", "PREPAID", "MODE"}}
	for _, e := range events {
		rate, term, prepaid, mode := "", "", "", ""
		if e.NewRate != nil {
			rate = e.NewRate.String()
		}
		if e.NewTermMonths != nil {
			term = fmt.Sprint(*e.NewTermMonths)
		}
		if e.PrepaidAmount != nil {
			prepaid = FormatMoney(*e.PrepaidAmount, loc)
		}
		if e.PrepayMode != nil {
			mode = string(*e.PrepayMode)
		}
		t.Body = append(t.Body, []string{string(e.Kind), FormatDate(e.EventDate, loc), rate, term, prepaid, mode})
	}
	return t
}

// WriteLoan renders one loan's overview and, when present, its events.
func WriteLoan(w io.Writer, f Format, loc domain.Locale, v LoanView) error {
	if f == JSON {
		return WriteJSON(w, loanDTO(v))
	}
	if err := WriteTable(w, loanRows(v, loc), false); err != nil {
		return err
	}
	if len(v.Events) == 0 {
		return nil
	}
	return WriteTable(w, loanEventRows(v.Events, loc), false)
}

// InstallmentDTO is the --json shape of one installment.
type InstallmentDTO struct {
	Due       string `json:"due"`
	Principal int64  `json:"principal"`
	Interest  int64  `json:"interest"`
	Fees      int64  `json:"fees"`
	Remaining int64  `json:"remaining"`
}

func installmentDTO(i domain.Installment) InstallmentDTO {
	return InstallmentDTO{
		Due: i.Due.String(), Principal: i.Principal.Minor(), Interest: i.Interest.Minor(),
		Fees: i.Fees.Minor(), Remaining: i.Remaining.Minor(),
	}
}

// ScheduleDTO is the --json shape of one schedule version.
type ScheduleDTO struct {
	Origin       string           `json:"origin"`
	RateBP       int64            `json:"rate_bp"`
	ValidFrom    string           `json:"valid_from"`
	Installments []InstallmentDTO `json:"installments"`
}

func scheduleDTO(s domain.Schedule) ScheduleDTO {
	out := ScheduleDTO{Origin: string(s.Origin), RateBP: s.Rate.BasisPoints(), ValidFrom: s.ValidFrom.String()}
	for _, i := range s.Installments {
		out.Installments = append(out.Installments, installmentDTO(i))
	}
	return out
}

func scheduleRows(s domain.Schedule, loc domain.Locale) Rows {
	t := Rows{Header: []string{"DUE", "PRINCIPAL", "INTEREST", "FEES", "REMAINING"}, MoneyCols: []int{1, 2, 3, 4}}
	for _, i := range s.Installments {
		t.Body = append(t.Body, []string{
			FormatDate(i.Due, loc), FormatMoney(i.Principal, loc), FormatMoney(i.Interest, loc),
			FormatMoney(i.Fees, loc), FormatMoney(i.Remaining, loc),
		})
	}
	return t
}

// WriteLoanSchedule renders one schedule version with its installments.
func WriteLoanSchedule(w io.Writer, f Format, loc domain.Locale, s domain.Schedule) error {
	if f == JSON {
		return WriteJSON(w, scheduleDTO(s))
	}
	return WriteTable(w, scheduleRows(s, loc), false)
}

// LoanConfirmDTO is the --json shape of `loan confirm`.
type LoanConfirmDTO struct {
	Transaction  TransactionDTO `json:"transaction"`
	Warnings     []string       `json:"warnings,omitempty"`
	ProposedRate string         `json:"proposed_rate,omitempty"`
}

// WriteLoanConfirm renders the transaction `loan confirm` booked, any schedule-mismatch warnings
// and the rate that would have produced the actual debited amount, when the service proposed one.
func WriteLoanConfirm(
	w io.Writer, f Format, loc domain.Locale, color bool,
	warnings []string, proposedRate domain.AnnualRate, tx app.TxView,
) error {
	if f == JSON {
		dto := LoanConfirmDTO{Transaction: transactionDTO(tx), Warnings: warnings}
		if proposedRate != 0 {
			dto.ProposedRate = proposedRate.String()
		}
		return WriteJSON(w, dto)
	}
	for _, msg := range warnings {
		if _, err := fmt.Fprintln(w, "Warning: "+msg); err != nil {
			return fmt.Errorf("write loan confirm warning: %w", err)
		}
	}
	if proposedRate != 0 {
		_, err := fmt.Fprintf(w, "Proposed rate: %s (not applied; record a rate_change event to adopt it)\n", proposedRate)
		if err != nil {
			return fmt.Errorf("write loan confirm proposed rate: %w", err)
		}
	}
	return WriteTx(w, f, loc, color, tx)
}
