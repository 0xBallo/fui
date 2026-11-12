package fuiproto

import (
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"
)

var (
	// ErrStructuralViolation marks violations that invalidate the whole stream.
	ErrStructuralViolation = errors.New("structural stream violation")
	// ErrTruncatedStream marks streams missing an end record.
	ErrTruncatedStream = errors.New("truncated stream")
)

// StructuralError is returned when stream-level invariants are violated.
type StructuralError struct {
	Line    int
	Message string
}

// Error implements the error interface.
func (e *StructuralError) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Message)
}

// Unwrap marks StructuralError as ErrStructuralViolation.
func (e *StructuralError) Unwrap() error {
	if e.Message == "missing end record" {
		return ErrTruncatedStream
	}
	return ErrStructuralViolation
}

// InvalidRow is a non-structural validation issue tied to one record.
type InvalidRow struct {
	Line    int
	Type    RecordType
	Message string
}

// ValidationResult is the output of stream validation.
type ValidationResult struct {
	Records   []Record
	RowErrors []InvalidRow
	End       EndRecord
}

// ValidateDate validates a YYYY-MM-DD date string.
func ValidateDate(value string) error {
	if value == "" {
		return fmt.Errorf("date is required")
	}

	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return fmt.Errorf("invalid date %q: %w", value, err)
	}
	if parsed.Format("2006-01-02") != value {
		return fmt.Errorf("invalid date %q", value)
	}

	return nil
}

// ValidateStream validates protocol structure and per-row semantics.
func ValidateStream(records iter.Seq2[Record, error]) (ValidationResult, error) {
	state := newValidationState()
	result := ValidationResult{}
	line := 0

	for record, err := range records {
		line++
		if err != nil {
			return result, structural(line, "decode error: %v", err)
		}
		if state.seenEnd {
			return result, structural(line, "record found after end")
		}

		include, rowErr, structuralErr := state.accept(line, record)
		if structuralErr != nil {
			return result, structuralErr
		}
		if rowErr != nil {
			result.RowErrors = append(result.RowErrors, *rowErr)
			continue
		}
		if include {
			result.Records = append(result.Records, record)
		}
	}

	if !state.seenEnd {
		return result, structural(line+1, "missing end record")
	}

	result.End = state.end
	return result, nil
}

type validationState struct {
	seenMeta   bool
	seenEnd    bool
	accounts   map[string]AccountRecord
	schedules  map[string]struct{}
	txCount    int
	errorCount int
	end        EndRecord
}

func newValidationState() validationState {
	return validationState{
		accounts:  map[string]AccountRecord{},
		schedules: map[string]struct{}{},
	}
}

func (s *validationState) accept(line int, record Record) (bool, *InvalidRow, error) {
	if !s.seenMeta && record.Type != RecordTypeMeta {
		return false, nil, structural(line, "first record must be meta")
	}

	switch record.Type {
	case RecordTypeMeta:
		return s.acceptMeta(line, record)
	case RecordTypeAccount:
		return s.acceptAccount(line, record)
	case RecordTypeTransaction:
		return s.acceptTransaction(line, record)
	case RecordTypeBalance:
		return s.acceptBalance(line, record)
	case RecordTypeSchedule:
		return s.acceptSchedule(line, record)
	case RecordTypeInstallment:
		return s.acceptInstallment(line, record)
	case RecordTypeError:
		return s.acceptError(line, record)
	case RecordTypeEnd:
		return s.acceptEnd(line, record)
	default:
		return false, nil, structural(line, "unknown record type %q", record.Type)
	}
}

func (s *validationState) acceptMeta(line int, record Record) (bool, *InvalidRow, error) {
	if s.seenMeta {
		return false, nil, structural(line, "meta can appear only once")
	}
	if record.Meta == nil {
		return false, nil, structural(line, "meta payload is required")
	}
	if record.Meta.Protocol == "" || record.Meta.Connector == "" || record.Meta.Version == "" {
		return false, nil, structural(line, "meta requires protocol, connector, and version")
	}

	major, err := ParseProtocolMajor(record.Meta.Protocol)
	if err != nil {
		return false, nil, structural(line, "invalid protocol: %v", err)
	}
	if major != 1 {
		return false, nil, structural(line, "unsupported protocol major %d", major)
	}

	s.seenMeta = true
	return true, nil, nil
}

func (s *validationState) acceptAccount(line int, record Record) (bool, *InvalidRow, error) {
	if record.Account == nil {
		return false, nil, structural(line, "account payload is required")
	}
	if _, exists := s.accounts[record.Account.Ref]; exists {
		return false, nil, structural(line, "account %q declared more than once", record.Account.Ref)
	}

	if msg := validateAccount(*record.Account); msg != "" {
		return false, &InvalidRow{Line: line, Type: RecordTypeAccount, Message: msg}, nil
	}

	s.accounts[record.Account.Ref] = *record.Account
	return true, nil, nil
}

func (s *validationState) acceptTransaction(line int, record Record) (bool, *InvalidRow, error) {
	if record.Transaction == nil {
		return false, nil, structural(line, "transaction payload is required")
	}

	account, ok := s.accounts[record.Transaction.Account]
	if !ok {
		return false, nil, structural(line, "transaction references unknown account %q", record.Transaction.Account)
	}
	if record.Transaction.Currency != account.Currency {
		return false, nil, structural(line, "transaction currency %q does not match account currency %q", record.Transaction.Currency, account.Currency)
	}

	s.txCount++
	if msg := validateTransaction(*record.Transaction); msg != "" {
		return false, &InvalidRow{Line: line, Type: RecordTypeTransaction, Message: msg}, nil
	}

	return true, nil, nil
}

func (s *validationState) acceptBalance(line int, record Record) (bool, *InvalidRow, error) {
	if record.Balance == nil {
		return false, nil, structural(line, "balance payload is required")
	}
	if _, ok := s.accounts[record.Balance.Account]; !ok {
		return false, nil, structural(line, "balance references unknown account %q", record.Balance.Account)
	}
	if msg := validateBalance(*record.Balance); msg != "" {
		return false, &InvalidRow{Line: line, Type: RecordTypeBalance, Message: msg}, nil
	}
	return true, nil, nil
}

func (s *validationState) acceptSchedule(line int, record Record) (bool, *InvalidRow, error) {
	if record.Schedule == nil {
		return false, nil, structural(line, "schedule payload is required")
	}

	account, ok := s.accounts[record.Schedule.Account]
	if !ok {
		return false, nil, structural(line, "schedule references unknown account %q", record.Schedule.Account)
	}

	if msg := validateSchedule(*record.Schedule, account); msg != "" {
		return false, &InvalidRow{Line: line, Type: RecordTypeSchedule, Message: msg}, nil
	}

	s.schedules[record.Schedule.Account] = struct{}{}
	return true, nil, nil
}

func (s *validationState) acceptInstallment(line int, record Record) (bool, *InvalidRow, error) {
	if record.Installment == nil {
		return false, nil, structural(line, "installment payload is required")
	}
	if _, ok := s.accounts[record.Installment.Account]; !ok {
		return false, nil, structural(line, "installment references unknown account %q", record.Installment.Account)
	}
	if _, ok := s.schedules[record.Installment.Account]; !ok {
		return false, nil, structural(line, "installment for account %q appears before schedule", record.Installment.Account)
	}

	if msg := validateInstallment(*record.Installment); msg != "" {
		return false, &InvalidRow{Line: line, Type: RecordTypeInstallment, Message: msg}, nil
	}
	return true, nil, nil
}

func (s *validationState) acceptError(line int, record Record) (bool, *InvalidRow, error) {
	if record.Error == nil {
		return false, nil, structural(line, "error payload is required")
	}
	s.errorCount++
	if msg := validateErrorRecord(*record.Error); msg != "" {
		return false, &InvalidRow{Line: line, Type: RecordTypeError, Message: msg}, nil
	}
	return true, nil, nil
}

func (s *validationState) acceptEnd(line int, record Record) (bool, *InvalidRow, error) {
	if record.End == nil {
		return false, nil, structural(line, "end payload is required")
	}
	if record.End.Transactions != s.txCount || record.End.Errors != s.errorCount {
		return false, nil, structural(line, "end counters mismatch: got transactions=%d errors=%d, expected transactions=%d errors=%d", record.End.Transactions, record.End.Errors, s.txCount, s.errorCount)
	}

	s.seenEnd = true
	s.end = *record.End
	return true, nil, nil
}

func validateAccount(record AccountRecord) string {
	if record.Ref == "" {
		return "account ref is required"
	}
	if record.Name == "" {
		return "account name is required"
	}
	if record.Currency == "" {
		return "account currency is required"
	}
	if !isValidAccountKind(record.Kind) {
		return fmt.Sprintf("invalid account kind %q", record.Kind)
	}
	return ""
}

func validateTransaction(record TransactionRecord) string {
	if record.Account == "" {
		return "transaction account is required"
	}
	if record.ID == "" {
		return "transaction id is required"
	}
	if err := ValidateDate(record.Date); err != nil {
		return err.Error()
	}
	if record.Currency == "" {
		return "transaction currency is required"
	}
	if record.Description == "" {
		return "transaction description is required"
	}
	if !isValidStatus(record.Status) {
		return fmt.Sprintf("invalid transaction status %q", record.Status)
	}
	if record.Kind != "" && !isValidTransactionKind(record.Kind) {
		return fmt.Sprintf("invalid transaction kind %q", record.Kind)
	}
	if err := validateOptionalDate(record.OperationDate); err != nil {
		return fmt.Sprintf("invalid operation_date: %v", err)
	}
	if err := validateOptionalDate(record.ValueDate); err != nil {
		return fmt.Sprintf("invalid value_date: %v", err)
	}
	if record.Original != nil {
		if record.Original.Currency == "" {
			return "original currency is required when original is present"
		}
	}
	if msg := validateTransactionSplits(record); msg != "" {
		return msg
	}
	return ""
}

func validateTransactionSplits(record TransactionRecord) string {
	if len(record.Splits) == 0 {
		return ""
	}
	if len(record.Splits) < 2 {
		return fmt.Sprintf("transaction splits requires at least 2 rows, got %d", len(record.Splits))
	}
	var sum int64
	for i, row := range record.Splits {
		if strings.TrimSpace(row.Category) == "" {
			return fmt.Sprintf("transaction splits[%d].category is required", i)
		}
		if row.Amount == 0 {
			return fmt.Sprintf("transaction splits[%d].amount must not be zero", i)
		}
		sum += row.Amount
	}
	if sum != record.Amount {
		return fmt.Sprintf("transaction splits sum to %d, expected %d", sum, record.Amount)
	}
	return ""
}

func validateBalance(record BalanceRecord) string {
	if record.Account == "" {
		return "balance account is required"
	}
	if err := ValidateDate(record.Date); err != nil {
		return err.Error()
	}
	return ""
}

func validateSchedule(record ScheduleRecord, account AccountRecord) string {
	if record.Account == "" {
		return "schedule account is required"
	}
	if err := ValidateDate(record.ValidFrom); err != nil {
		return err.Error()
	}
	if !isValidRateType(record.RateType) {
		return fmt.Sprintf("invalid rate_type %q", record.RateType)
	}
	if account.Kind != AccountKindLoan {
		return fmt.Sprintf("schedule account kind must be %q", AccountKindLoan)
	}
	return ""
}

func validateInstallment(record InstallmentRecord) string {
	if record.Account == "" {
		return "installment account is required"
	}
	if err := ValidateDate(record.DueDate); err != nil {
		return err.Error()
	}
	return ""
}

func validateErrorRecord(record ErrorRecord) string {
	if record.Line <= 0 {
		return "error line must be > 0"
	}
	if record.Message == "" {
		return "error message is required"
	}
	return ""
}

func validateOptionalDate(value string) error {
	if value == "" {
		return nil
	}
	return ValidateDate(value)
}

func isValidAccountKind(kind AccountKind) bool {
	switch kind {
	case AccountKindChecking, AccountKindCard, AccountKindSavings, AccountKindInvestment, AccountKindCash, AccountKindLoan:
		return true
	default:
		return false
	}
}

func isValidStatus(status TransactionStatus) bool {
	switch status {
	case TransactionStatusBooked, TransactionStatusPending:
		return true
	default:
		return false
	}
}

func isValidTransactionKind(kind TransactionKind) bool {
	switch kind {
	case TransactionKindPayment, TransactionKindTransfer, TransactionKindExchange, TransactionKindFee, TransactionKindInterest, TransactionKindRefund, TransactionKindInvestment, TransactionKindOther:
		return true
	default:
		return false
	}
}

func isValidRateType(rateType RateType) bool {
	switch rateType {
	case RateTypeFixed, RateTypeVariable, RateTypeMixed:
		return true
	default:
		return false
	}
}

func structural(line int, format string, args ...any) error {
	return &StructuralError{
		Line:    line,
		Message: fmt.Sprintf(format, args...),
	}
}
