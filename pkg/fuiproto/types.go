// Package fuiproto defines the public fui/1 protocol model, codec, and validator.
package fuiproto

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const (
	// ProtocolFUI1 is the current fui protocol version.
	ProtocolFUI1 = "fui/1"

	// ExitCodeOK is the successful connector exit code.
	ExitCodeOK = 0
	// ExitCodeBadConfig indicates invalid connector configuration.
	ExitCodeBadConfig = 2
	// ExitCodeUnreadableSource indicates unreadable input source or unknown source format.
	ExitCodeUnreadableSource = 3

	// MaxLineBytes is the maximum accepted JSONL record size.
	MaxLineBytes = 1 << 20
)

// RecordType identifies a fui/1 stream record variant.
type RecordType string

const (
	// RecordTypeMeta identifies meta records.
	RecordTypeMeta RecordType = "meta"
	// RecordTypeAccount identifies account records.
	RecordTypeAccount RecordType = "account"
	// RecordTypeTransaction identifies transaction records.
	RecordTypeTransaction RecordType = "transaction"
	// RecordTypeBalance identifies balance records.
	RecordTypeBalance RecordType = "balance"
	// RecordTypeSchedule identifies schedule records.
	RecordTypeSchedule RecordType = "schedule"
	// RecordTypeInstallment identifies installment records.
	RecordTypeInstallment RecordType = "installment"
	// RecordTypeError identifies error records.
	RecordTypeError RecordType = "error"
	// RecordTypeEnd identifies end records.
	RecordTypeEnd RecordType = "end"
)

// AccountKind is the account kind enum.
type AccountKind string

const (
	// AccountKindChecking is a checking account.
	AccountKindChecking AccountKind = "checking"
	// AccountKindCard is a card account.
	AccountKindCard AccountKind = "card"
	// AccountKindSavings is a savings account.
	AccountKindSavings AccountKind = "savings"
	// AccountKindInvestment is an investment account.
	AccountKindInvestment AccountKind = "investment"
	// AccountKindCash is a cash account.
	AccountKindCash AccountKind = "cash"
	// AccountKindLoan is a loan account.
	AccountKindLoan AccountKind = "loan"
)

// TransactionStatus is the transaction status enum.
type TransactionStatus string

const (
	// TransactionStatusBooked identifies booked transactions.
	TransactionStatusBooked TransactionStatus = "booked"
	// TransactionStatusPending identifies pending transactions.
	TransactionStatusPending TransactionStatus = "pending"
)

// TransactionKind is the transaction kind enum.
type TransactionKind string

const (
	// TransactionKindPayment identifies payment transactions.
	TransactionKindPayment TransactionKind = "payment"
	// TransactionKindTransfer identifies transfer transactions.
	TransactionKindTransfer TransactionKind = "transfer"
	// TransactionKindExchange identifies exchange transactions.
	TransactionKindExchange TransactionKind = "exchange"
	// TransactionKindFee identifies fee transactions.
	TransactionKindFee TransactionKind = "fee"
	// TransactionKindInterest identifies interest transactions.
	TransactionKindInterest TransactionKind = "interest"
	// TransactionKindRefund identifies refund transactions.
	TransactionKindRefund TransactionKind = "refund"
	// TransactionKindInvestment identifies investment transactions.
	TransactionKindInvestment TransactionKind = "investment"
	// TransactionKindOther identifies unclassified transactions.
	TransactionKindOther TransactionKind = "other"
)

// RateType is the schedule rate type enum.
type RateType string

const (
	// RateTypeFixed identifies fixed rates.
	RateTypeFixed RateType = "fixed"
	// RateTypeVariable identifies variable rates.
	RateTypeVariable RateType = "variable"
	// RateTypeMixed identifies mixed rates.
	RateTypeMixed RateType = "mixed"
)

// SourceMeta contains source file metadata for meta records.
type SourceMeta struct {
	File   string `json:"file,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
}

// MetaRecord is the stream metadata record.
type MetaRecord struct {
	Protocol  string     `json:"protocol"`
	Connector string     `json:"connector"`
	Version   string     `json:"version"`
	Source    SourceMeta `json:"source"`
}

// AccountRecord declares an account used by subsequent records.
type AccountRecord struct {
	Ref      string      `json:"ref"`
	Name     string      `json:"name"`
	Currency string      `json:"currency"`
	Kind     AccountKind `json:"kind"`
	IBAN     string      `json:"iban,omitempty"`
}

// OriginalAmount represents the original transaction amount/currency.
type OriginalAmount struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

// TransactionRecord is the transaction record.
type TransactionRecord struct {
	Account          string            `json:"account"`
	ID               string            `json:"id"`
	Date             string            `json:"date"`
	OperationDate    string            `json:"operation_date,omitempty"`
	ValueDate        string            `json:"value_date,omitempty"`
	Amount           int64             `json:"amount"`
	Currency         string            `json:"currency"`
	Description      string            `json:"description"`
	Payee            string            `json:"payee,omitempty"`
	CounterpartyIBAN string            `json:"counterparty_iban,omitempty"`
	Status           TransactionStatus `json:"status"`
	Kind             TransactionKind   `json:"kind,omitempty"`
	Transfer         string            `json:"transfer,omitempty"`
	Original         *OriginalAmount   `json:"original,omitempty"`
	BalanceAfter     int64             `json:"balance_after,omitempty"`
	CategoryHint     string            `json:"category_hint,omitempty"`
	Category         string            `json:"category,omitempty"`
	Tags             []string          `json:"tags,omitempty"`
	Notes            string            `json:"notes,omitempty"`
	Raw              map[string]any    `json:"raw,omitempty"`
}

// BalanceRecord is an optional reconciliation balance.
type BalanceRecord struct {
	Account string `json:"account"`
	Date    string `json:"date"`
	Amount  int64  `json:"amount"`
}

// ScheduleRecord declares loan schedule metadata.
type ScheduleRecord struct {
	Account   string   `json:"account"`
	ValidFrom string   `json:"valid_from"`
	Rate      int64    `json:"rate"`
	RateType  RateType `json:"rate_type"`
}

// InstallmentRecord contains one installment.
type InstallmentRecord struct {
	Account   string `json:"account"`
	DueDate   string `json:"due_date"`
	Principal int64  `json:"principal"`
	Interest  int64  `json:"interest"`
	Fees      int64  `json:"fees"`
	Remaining int64  `json:"remaining"`
}

// ErrorRecord reports an unconvertible source row.
type ErrorRecord struct {
	Line    int            `json:"line"`
	Message string         `json:"message"`
	Raw     map[string]any `json:"raw,omitempty"`
}

// EndRecord closes a stream and reports totals.
type EndRecord struct {
	Transactions int `json:"transactions"`
	Errors       int `json:"errors"`
}

// Record is a tagged union over all fui/1 record variants.
type Record struct {
	Type        RecordType
	Meta        *MetaRecord
	Account     *AccountRecord
	Transaction *TransactionRecord
	Balance     *BalanceRecord
	Schedule    *ScheduleRecord
	Installment *InstallmentRecord
	Error       *ErrorRecord
	End         *EndRecord
}

// ConfigParam describes one connector configuration field.
type ConfigParam struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// DescribeResponse is the output payload for connector describe.
type DescribeResponse struct {
	Protocol          string        `json:"protocol"`
	Name              string        `json:"name"`
	Version           string        `json:"version"`
	AcceptedFileTypes []string      `json:"accepted_file_types,omitempty"`
	ConfigParams      []ConfigParam `json:"config_params,omitempty"`
}

// FetchRequest is the stdin payload for connector fetch.
type FetchRequest struct {
	Protocol string         `json:"protocol"`
	Config   map[string]any `json:"config,omitempty"`
	File     *string        `json:"file"`
	Since    *string        `json:"since,omitempty"`
}

// ParseProtocolMajor extracts the protocol major component from strings like "fui/1".
func ParseProtocolMajor(protocol string) (int, error) {
	parts := strings.Split(protocol, "/")
	if len(parts) != 2 || parts[0] != "fui" {
		return 0, fmt.Errorf("invalid protocol %q", protocol)
	}

	major, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid protocol major in %q: %w", protocol, err)
	}

	return major, nil
}

// MarshalJSON marshals the concrete record payload according to the type tag.
func (r Record) MarshalJSON() ([]byte, error) {
	switch r.Type {
	case RecordTypeMeta:
		return marshalMetaRecord(r.Type, r.Meta)
	case RecordTypeAccount:
		return marshalAccountRecord(r.Type, r.Account)
	case RecordTypeTransaction:
		return marshalTransactionRecord(r.Type, r.Transaction)
	case RecordTypeBalance:
		return marshalBalanceRecord(r.Type, r.Balance)
	case RecordTypeSchedule:
		return marshalScheduleRecord(r.Type, r.Schedule)
	case RecordTypeInstallment:
		return marshalInstallmentRecord(r.Type, r.Installment)
	case RecordTypeError:
		return marshalErrorRecord(r.Type, r.Error)
	case RecordTypeEnd:
		return marshalEndRecord(r.Type, r.End)
	default:
		return nil, fmt.Errorf("unknown record type %q", r.Type)
	}
}

// UnmarshalJSON unmarshals one concrete record payload from a type-tagged object.
func (r *Record) UnmarshalJSON(data []byte) error {
	var header struct {
		Type RecordType `json:"type"`
	}

	if err := json.Unmarshal(data, &header); err != nil {
		return fmt.Errorf("decode record header: %w", err)
	}

	switch header.Type {
	case RecordTypeMeta:
		return unmarshalMetaRecord(data, r)
	case RecordTypeAccount:
		return unmarshalAccountRecord(data, r)
	case RecordTypeTransaction:
		return unmarshalTransactionRecord(data, r)
	case RecordTypeBalance:
		return unmarshalBalanceRecord(data, r)
	case RecordTypeSchedule:
		return unmarshalScheduleRecord(data, r)
	case RecordTypeInstallment:
		return unmarshalInstallmentRecord(data, r)
	case RecordTypeError:
		return unmarshalErrorRecord(data, r)
	case RecordTypeEnd:
		return unmarshalEndRecord(data, r)
	default:
		return fmt.Errorf("unknown record type %q", header.Type)
	}
}

func marshalMetaRecord(recordType RecordType, record *MetaRecord) ([]byte, error) {
	if record == nil {
		return nil, fmt.Errorf("meta payload is required for type %q", recordType)
	}
	payload := struct {
		Type RecordType `json:"type"`
		MetaRecord
	}{Type: recordType, MetaRecord: *record}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode meta record: %w", err)
	}
	return data, nil
}

func marshalAccountRecord(recordType RecordType, record *AccountRecord) ([]byte, error) {
	if record == nil {
		return nil, fmt.Errorf("account payload is required for type %q", recordType)
	}
	payload := struct {
		Type RecordType `json:"type"`
		AccountRecord
	}{Type: recordType, AccountRecord: *record}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode account record: %w", err)
	}
	return data, nil
}

func marshalTransactionRecord(recordType RecordType, record *TransactionRecord) ([]byte, error) {
	if record == nil {
		return nil, fmt.Errorf("transaction payload is required for type %q", recordType)
	}
	payload := struct {
		Type RecordType `json:"type"`
		TransactionRecord
	}{Type: recordType, TransactionRecord: *record}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode transaction record: %w", err)
	}
	return data, nil
}

func marshalBalanceRecord(recordType RecordType, record *BalanceRecord) ([]byte, error) {
	if record == nil {
		return nil, fmt.Errorf("balance payload is required for type %q", recordType)
	}
	payload := struct {
		Type RecordType `json:"type"`
		BalanceRecord
	}{Type: recordType, BalanceRecord: *record}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode balance record: %w", err)
	}
	return data, nil
}

func marshalScheduleRecord(recordType RecordType, record *ScheduleRecord) ([]byte, error) {
	if record == nil {
		return nil, fmt.Errorf("schedule payload is required for type %q", recordType)
	}
	payload := struct {
		Type RecordType `json:"type"`
		ScheduleRecord
	}{Type: recordType, ScheduleRecord: *record}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode schedule record: %w", err)
	}
	return data, nil
}

func marshalInstallmentRecord(recordType RecordType, record *InstallmentRecord) ([]byte, error) {
	if record == nil {
		return nil, fmt.Errorf("installment payload is required for type %q", recordType)
	}
	payload := struct {
		Type RecordType `json:"type"`
		InstallmentRecord
	}{Type: recordType, InstallmentRecord: *record}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode installment record: %w", err)
	}
	return data, nil
}

func marshalErrorRecord(recordType RecordType, record *ErrorRecord) ([]byte, error) {
	if record == nil {
		return nil, fmt.Errorf("error payload is required for type %q", recordType)
	}
	payload := struct {
		Type RecordType `json:"type"`
		ErrorRecord
	}{Type: recordType, ErrorRecord: *record}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode error record: %w", err)
	}
	return data, nil
}

func marshalEndRecord(recordType RecordType, record *EndRecord) ([]byte, error) {
	if record == nil {
		return nil, fmt.Errorf("end payload is required for type %q", recordType)
	}
	payload := struct {
		Type RecordType `json:"type"`
		EndRecord
	}{Type: recordType, EndRecord: *record}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode end record: %w", err)
	}
	return data, nil
}

func unmarshalMetaRecord(data []byte, out *Record) error {
	var payload struct {
		Type RecordType `json:"type"`
		MetaRecord
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode meta record: %w", err)
	}
	record := payload.MetaRecord
	*out = Record{Type: payload.Type, Meta: &record}
	return nil
}

func unmarshalAccountRecord(data []byte, out *Record) error {
	var payload struct {
		Type RecordType `json:"type"`
		AccountRecord
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode account record: %w", err)
	}
	record := payload.AccountRecord
	*out = Record{Type: payload.Type, Account: &record}
	return nil
}

func unmarshalTransactionRecord(data []byte, out *Record) error {
	var payload struct {
		Type RecordType `json:"type"`
		TransactionRecord
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode transaction record: %w", err)
	}
	record := payload.TransactionRecord
	*out = Record{Type: payload.Type, Transaction: &record}
	return nil
}

func unmarshalBalanceRecord(data []byte, out *Record) error {
	var payload struct {
		Type RecordType `json:"type"`
		BalanceRecord
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode balance record: %w", err)
	}
	record := payload.BalanceRecord
	*out = Record{Type: payload.Type, Balance: &record}
	return nil
}

func unmarshalScheduleRecord(data []byte, out *Record) error {
	var payload struct {
		Type RecordType `json:"type"`
		ScheduleRecord
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode schedule record: %w", err)
	}
	record := payload.ScheduleRecord
	*out = Record{Type: payload.Type, Schedule: &record}
	return nil
}

func unmarshalInstallmentRecord(data []byte, out *Record) error {
	var payload struct {
		Type RecordType `json:"type"`
		InstallmentRecord
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode installment record: %w", err)
	}
	record := payload.InstallmentRecord
	*out = Record{Type: payload.Type, Installment: &record}
	return nil
}

func unmarshalErrorRecord(data []byte, out *Record) error {
	var payload struct {
		Type RecordType `json:"type"`
		ErrorRecord
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode error record: %w", err)
	}
	record := payload.ErrorRecord
	*out = Record{Type: payload.Type, Error: &record}
	return nil
}

func unmarshalEndRecord(data []byte, out *Record) error {
	var payload struct {
		Type RecordType `json:"type"`
		EndRecord
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode end record: %w", err)
	}
	record := payload.EndRecord
	*out = Record{Type: payload.Type, End: &record}
	return nil
}
