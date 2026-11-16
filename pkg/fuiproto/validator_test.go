package fuiproto

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateStreamStructuralRules(t *testing.T) {
	base := validStreamRecords()

	tests := []struct {
		name       string
		records    []Record
		wantErrSub string
		wantTrunc  bool
	}{
		{
			name: "meta must be first",
			records: []Record{
				base[1], base[0], base[3],
			},
			wantErrSub: "first record must be meta",
		},
		{
			name: "different protocol major rejected",
			records: []Record{
				{
					Type: RecordTypeMeta,
					Meta: &MetaRecord{
						Protocol:  "fui/2",
						Connector: "x",
						Version:   "0.1.0",
					},
				},
				base[3],
			},
			wantErrSub: "unsupported protocol major",
		},
		{
			name: "account must be declared before use",
			records: []Record{
				base[0],
				{
					Type: RecordTypeTransaction,
					Transaction: &TransactionRecord{
						Account:     "missing",
						ID:          "tx-1",
						Date:        "2026-10-01",
						Amount:      1,
						Currency:    "EUR",
						Description: "x",
						Status:      TransactionStatusBooked,
					},
				},
				base[3],
			},
			wantErrSub: "unknown account",
		},
		{
			name: "transaction currency must match account",
			records: []Record{
				base[0], base[1],
				{
					Type: RecordTypeTransaction,
					Transaction: &TransactionRecord{
						Account:     "acc-1",
						ID:          "tx-1",
						Date:        "2026-10-01",
						Amount:      1,
						Currency:    "USD",
						Description: "x",
						Status:      TransactionStatusBooked,
					},
				},
				base[3],
			},
			wantErrSub: "does not match account currency",
		},
		{
			name: "schedule must appear before installment",
			records: []Record{
				base[0],
				{
					Type: RecordTypeAccount,
					Account: &AccountRecord{
						Ref:      "loan-1",
						Name:     "Loan",
						Currency: "EUR",
						Kind:     AccountKindLoan,
					},
				},
				{
					Type: RecordTypeInstallment,
					Installment: &InstallmentRecord{
						Account:   "loan-1",
						DueDate:   "2026-10-15",
						Principal: 100,
						Interest:  10,
						Fees:      0,
						Remaining: 1000,
					},
				},
				{
					Type: RecordTypeEnd,
					End: &EndRecord{
						Transactions: 0,
						Errors:       0,
					},
				},
			},
			wantErrSub: "appears before schedule",
		},
		{
			name: "end counters must match",
			records: []Record{
				base[0], base[1], base[2],
				{
					Type: RecordTypeEnd,
					End: &EndRecord{
						Transactions: 2,
						Errors:       0,
					},
				},
			},
			wantErrSub: "end counters mismatch",
		},
		{
			name: "nothing after end",
			records: []Record{
				base[0],
				base[1],
				{
					Type: RecordTypeEnd,
					End: &EndRecord{
						Transactions: 0,
						Errors:       0,
					},
				},
				base[2],
			},
			wantErrSub: "record found after end",
		},
		{
			name: "missing end is truncated",
			records: []Record{
				base[0], base[1], base[2],
			},
			wantErrSub: "missing end record",
			wantTrunc:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidateStream(seqFromRecords(tc.records))
			if err == nil {
				t.Fatalf("expected structural error")
			}
			if !strings.Contains(err.Error(), tc.wantErrSub) {
				t.Fatalf("error = %v, want substring %q", err, tc.wantErrSub)
			}
			if !errors.Is(err, ErrStructuralViolation) && !errors.Is(err, ErrTruncatedStream) {
				t.Fatalf("error = %v, want structural marker", err)
			}
			if tc.wantTrunc && !errors.Is(err, ErrTruncatedStream) {
				t.Fatalf("error = %v, want ErrTruncatedStream", err)
			}
		})
	}
}

func TestValidateStreamRowLevelRules(t *testing.T) {
	tests := []struct {
		name    string
		records []Record
		wantRow RecordType
		wantErr string
	}{
		{
			name: "required account fields",
			records: []Record{
				{
					Type: RecordTypeMeta,
					Meta: &MetaRecord{Protocol: ProtocolFUI1, Connector: "x", Version: "0.1.0"},
				},
				{
					Type: RecordTypeAccount,
					Account: &AccountRecord{
						Name:     "Account",
						Currency: "EUR",
						Kind:     AccountKindChecking,
					},
				},
				{
					Type: RecordTypeEnd,
					End:  &EndRecord{},
				},
			},
			wantRow: RecordTypeAccount,
			wantErr: "required",
		},
		{
			name: "invalid account kind enum",
			records: []Record{
				{
					Type: RecordTypeMeta,
					Meta: &MetaRecord{Protocol: ProtocolFUI1, Connector: "x", Version: "0.1.0"},
				},
				{
					Type: RecordTypeAccount,
					Account: &AccountRecord{
						Ref:      "acc",
						Name:     "Account",
						Currency: "EUR",
						Kind:     AccountKind("wallet"),
					},
				},
				{
					Type: RecordTypeEnd,
					End:  &EndRecord{},
				},
			},
			wantRow: RecordTypeAccount,
			wantErr: "invalid account kind",
		},
		{
			name: "invalid transaction status enum",
			records: []Record{
				{
					Type: RecordTypeMeta,
					Meta: &MetaRecord{Protocol: ProtocolFUI1, Connector: "x", Version: "0.1.0"},
				},
				{
					Type: RecordTypeAccount,
					Account: &AccountRecord{
						Ref:      "acc",
						Name:     "Account",
						Currency: "EUR",
						Kind:     AccountKindChecking,
					},
				},
				{
					Type: RecordTypeTransaction,
					Transaction: &TransactionRecord{
						Account:     "acc",
						ID:          "tx",
						Date:        "2026-10-01",
						Amount:      1,
						Currency:    "EUR",
						Description: "x",
						Status:      TransactionStatus("done"),
					},
				},
				{
					Type: RecordTypeEnd,
					End: &EndRecord{
						Transactions: 1,
					},
				},
			},
			wantRow: RecordTypeTransaction,
			wantErr: "invalid transaction status",
		},
		{
			name: "invalid transaction kind enum",
			records: []Record{
				{
					Type: RecordTypeMeta,
					Meta: &MetaRecord{Protocol: ProtocolFUI1, Connector: "x", Version: "0.1.0"},
				},
				{
					Type: RecordTypeAccount,
					Account: &AccountRecord{
						Ref:      "acc",
						Name:     "Account",
						Currency: "EUR",
						Kind:     AccountKindChecking,
					},
				},
				{
					Type: RecordTypeTransaction,
					Transaction: &TransactionRecord{
						Account:     "acc",
						ID:          "tx",
						Date:        "2026-10-01",
						Amount:      1,
						Currency:    "EUR",
						Description: "x",
						Status:      TransactionStatusBooked,
						Kind:        TransactionKind("wire"),
					},
				},
				{
					Type: RecordTypeEnd,
					End: &EndRecord{
						Transactions: 1,
					},
				},
			},
			wantRow: RecordTypeTransaction,
			wantErr: "invalid transaction kind",
		},
		{
			name: "invalid date format",
			records: []Record{
				{
					Type: RecordTypeMeta,
					Meta: &MetaRecord{Protocol: ProtocolFUI1, Connector: "x", Version: "0.1.0"},
				},
				{
					Type: RecordTypeAccount,
					Account: &AccountRecord{
						Ref:      "acc",
						Name:     "Account",
						Currency: "EUR",
						Kind:     AccountKindChecking,
					},
				},
				{
					Type: RecordTypeTransaction,
					Transaction: &TransactionRecord{
						Account:     "acc",
						ID:          "tx",
						Date:        "01-10-2026",
						Amount:      1,
						Currency:    "EUR",
						Description: "x",
						Status:      TransactionStatusBooked,
					},
				},
				{
					Type: RecordTypeEnd,
					End: &EndRecord{
						Transactions: 1,
					},
				},
			},
			wantRow: RecordTypeTransaction,
			wantErr: "invalid date",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ValidateStream(seqFromRecords(tc.records))
			if err != nil {
				t.Fatalf("unexpected structural error: %v", err)
			}
			if len(result.RowErrors) != 1 {
				t.Fatalf("row errors = %d, want 1 (%v)", len(result.RowErrors), result.RowErrors)
			}
			if result.RowErrors[0].Type != tc.wantRow {
				t.Fatalf("row error type = %q, want %q", result.RowErrors[0].Type, tc.wantRow)
			}
			if !strings.Contains(result.RowErrors[0].Message, tc.wantErr) {
				t.Fatalf("row error = %q, want substring %q", result.RowErrors[0].Message, tc.wantErr)
			}
		})
	}
}

func TestValidateStreamSupportsScheduleInstallmentAndErrorRecords(t *testing.T) {
	records := []Record{
		{
			Type: RecordTypeMeta,
			Meta: &MetaRecord{
				Protocol:  ProtocolFUI1,
				Connector: "x",
				Version:   "0.1.0",
			},
		},
		{
			Type: RecordTypeAccount,
			Account: &AccountRecord{
				Ref:      "loan-1",
				Name:     "Loan",
				Currency: "EUR",
				Kind:     AccountKindLoan,
			},
		},
		{
			Type: RecordTypeAccount,
			Account: &AccountRecord{
				Ref:      "acc-1",
				Name:     "Main",
				Currency: "EUR",
				Kind:     AccountKindChecking,
			},
		},
		{
			Type: RecordTypeSchedule,
			Schedule: &ScheduleRecord{
				Account:   "loan-1",
				ValidFrom: "2026-01-01",
				Rate:      100,
				RateType:  RateTypeFixed,
			},
		},
		{
			Type: RecordTypeInstallment,
			Installment: &InstallmentRecord{
				Account:   "loan-1",
				DueDate:   "2026-11-15",
				Principal: 1000,
				Interest:  10,
				Fees:      0,
				Remaining: 9000,
			},
		},
		{
			Type: RecordTypeBalance,
			Balance: &BalanceRecord{
				Account: "acc-1",
				Date:    "2026-11-11",
				Amount:  1000,
			},
		},
		{
			Type: RecordTypeSecurityOp,
			SecurityOp: &SecurityOpRecord{
				Account:   "acc-1",
				Date:      "2026-11-12",
				Kind:      SecurityOpKindBuy,
				Quantity:  1000000,
				UnitPrice: 1234,
				Fees:      10,
				CashTx:    "tx-1",
				Security: SecurityMeta{
					Name:          "World ETF",
					Ticker:        "VWCE",
					Kind:          SecurityKindETF,
					Currency:      "EUR",
					QuantityScale: 6,
				},
			},
		},
		{
			Type: RecordTypeError,
			Error: &ErrorRecord{
				Line:    7,
				Message: "source row invalid",
			},
		},
		{
			Type: RecordTypeEnd,
			End: &EndRecord{
				Transactions: 0,
				Errors:       1,
			},
		},
	}

	result, err := ValidateStream(seqFromRecords(records))
	if err != nil {
		t.Fatalf("unexpected structural error: %v", err)
	}
	if len(result.RowErrors) != 0 {
		t.Fatalf("unexpected row errors: %v", result.RowErrors)
	}
	if result.End.Errors != 1 {
		t.Fatalf("end.errors = %d, want 1", result.End.Errors)
	}
}

func TestValidateDate(t *testing.T) {
	if err := ValidateDate("2026-11-11"); err != nil {
		t.Fatalf("unexpected valid date error: %v", err)
	}
	if err := ValidateDate(""); err == nil {
		t.Fatalf("expected empty date error")
	}
	if err := ValidateDate("11-11-2026"); err == nil {
		t.Fatalf("expected invalid format error")
	}
}

func TestValidateStreamInvalidTransactionStillCountsForEnd(t *testing.T) {
	records := []Record{
		{
			Type: RecordTypeMeta,
			Meta: &MetaRecord{
				Protocol:  ProtocolFUI1,
				Connector: "x",
				Version:   "0.1.0",
			},
		},
		{
			Type: RecordTypeAccount,
			Account: &AccountRecord{
				Ref:      "acc",
				Name:     "Account",
				Currency: "EUR",
				Kind:     AccountKindChecking,
			},
		},
		{
			Type: RecordTypeTransaction,
			Transaction: &TransactionRecord{
				Account:     "acc",
				ID:          "tx-1",
				Date:        "2026-11-12",
				Amount:      1,
				Currency:    "EUR",
				Description: "desc",
				Status:      TransactionStatus("invalid"),
			},
		},
		{
			Type: RecordTypeEnd,
			End: &EndRecord{
				Transactions: 1,
				Errors:       0,
			},
		},
	}

	result, err := ValidateStream(seqFromRecords(records))
	if err != nil {
		t.Fatalf("unexpected structural error: %v", err)
	}
	if len(result.RowErrors) != 1 {
		t.Fatalf("row errors = %d, want 1", len(result.RowErrors))
	}
	if result.RowErrors[0].Type != RecordTypeTransaction {
		t.Fatalf("row error type = %q, want %q", result.RowErrors[0].Type, RecordTypeTransaction)
	}
}

func TestValidateStreamAdditionalBranches(t *testing.T) {
	tests := []struct {
		name        string
		records     []Record
		structural  string
		rowContains string
	}{
		{
			name: "duplicate account structural",
			records: []Record{
				{Type: RecordTypeMeta, Meta: &MetaRecord{Protocol: ProtocolFUI1, Connector: "x", Version: "0.1.0"}},
				{Type: RecordTypeAccount, Account: &AccountRecord{Ref: "a", Name: "A", Currency: "EUR", Kind: AccountKindChecking}},
				{Type: RecordTypeAccount, Account: &AccountRecord{Ref: "a", Name: "B", Currency: "EUR", Kind: AccountKindChecking}},
				{Type: RecordTypeEnd, End: &EndRecord{}},
			},
			structural: "declared more than once",
		},
		{
			name: "meta missing required fields structural",
			records: []Record{
				{Type: RecordTypeMeta, Meta: &MetaRecord{Protocol: ProtocolFUI1}},
				{Type: RecordTypeEnd, End: &EndRecord{}},
			},
			structural: "meta requires protocol",
		},
		{
			name: "balance invalid date row-level",
			records: []Record{
				{Type: RecordTypeMeta, Meta: &MetaRecord{Protocol: ProtocolFUI1, Connector: "x", Version: "0.1.0"}},
				{Type: RecordTypeAccount, Account: &AccountRecord{Ref: "a", Name: "A", Currency: "EUR", Kind: AccountKindChecking}},
				{Type: RecordTypeBalance, Balance: &BalanceRecord{Account: "a", Date: "20261111", Amount: 10}},
				{Type: RecordTypeEnd, End: &EndRecord{}},
			},
			rowContains: "invalid date",
		},
		{
			name: "schedule invalid rate type row-level",
			records: []Record{
				{Type: RecordTypeMeta, Meta: &MetaRecord{Protocol: ProtocolFUI1, Connector: "x", Version: "0.1.0"}},
				{Type: RecordTypeAccount, Account: &AccountRecord{Ref: "loan", Name: "Loan", Currency: "EUR", Kind: AccountKindLoan}},
				{Type: RecordTypeSchedule, Schedule: &ScheduleRecord{Account: "loan", ValidFrom: "2026-01-01", Rate: 1, RateType: RateType("bad")}},
				{Type: RecordTypeEnd, End: &EndRecord{}},
			},
			rowContains: "invalid rate_type",
		},
		{
			name: "schedule account must be loan row-level",
			records: []Record{
				{Type: RecordTypeMeta, Meta: &MetaRecord{Protocol: ProtocolFUI1, Connector: "x", Version: "0.1.0"}},
				{Type: RecordTypeAccount, Account: &AccountRecord{Ref: "acc", Name: "A", Currency: "EUR", Kind: AccountKindChecking}},
				{Type: RecordTypeSchedule, Schedule: &ScheduleRecord{Account: "acc", ValidFrom: "2026-01-01", Rate: 1, RateType: RateTypeFixed}},
				{Type: RecordTypeEnd, End: &EndRecord{}},
			},
			rowContains: "must be \"loan\"",
		},
		{
			name: "installment invalid date row-level",
			records: []Record{
				{Type: RecordTypeMeta, Meta: &MetaRecord{Protocol: ProtocolFUI1, Connector: "x", Version: "0.1.0"}},
				{Type: RecordTypeAccount, Account: &AccountRecord{Ref: "loan", Name: "Loan", Currency: "EUR", Kind: AccountKindLoan}},
				{Type: RecordTypeSchedule, Schedule: &ScheduleRecord{Account: "loan", ValidFrom: "2026-01-01", Rate: 1, RateType: RateTypeFixed}},
				{Type: RecordTypeInstallment, Installment: &InstallmentRecord{Account: "loan", DueDate: "20260101", Principal: 1, Interest: 1, Fees: 0, Remaining: 1}},
				{Type: RecordTypeEnd, End: &EndRecord{}},
			},
			rowContains: "invalid date",
		},
		{
			name: "error record invalid line row-level",
			records: []Record{
				{Type: RecordTypeMeta, Meta: &MetaRecord{Protocol: ProtocolFUI1, Connector: "x", Version: "0.1.0"}},
				{Type: RecordTypeError, Error: &ErrorRecord{Line: 0, Message: "x"}},
				{Type: RecordTypeEnd, End: &EndRecord{Errors: 1}},
			},
			rowContains: "line must be > 0",
		},
		{
			name: "transaction original currency required",
			records: []Record{
				{Type: RecordTypeMeta, Meta: &MetaRecord{Protocol: ProtocolFUI1, Connector: "x", Version: "0.1.0"}},
				{Type: RecordTypeAccount, Account: &AccountRecord{Ref: "a", Name: "A", Currency: "EUR", Kind: AccountKindChecking}},
				{Type: RecordTypeTransaction, Transaction: &TransactionRecord{Account: "a", ID: "t", Date: "2026-11-11", Amount: 1, Currency: "EUR", Description: "x", Status: TransactionStatusBooked, Original: &OriginalAmount{Amount: 10}}},
				{Type: RecordTypeEnd, End: &EndRecord{Transactions: 1}},
			},
			rowContains: "original currency is required",
		},
		{
			name: "transaction operation date invalid",
			records: []Record{
				{Type: RecordTypeMeta, Meta: &MetaRecord{Protocol: ProtocolFUI1, Connector: "x", Version: "0.1.0"}},
				{Type: RecordTypeAccount, Account: &AccountRecord{Ref: "a", Name: "A", Currency: "EUR", Kind: AccountKindChecking}},
				{Type: RecordTypeTransaction, Transaction: &TransactionRecord{Account: "a", ID: "t", Date: "2026-11-11", OperationDate: "20261111", Amount: 1, Currency: "EUR", Description: "x", Status: TransactionStatusBooked}},
				{Type: RecordTypeEnd, End: &EndRecord{Transactions: 1}},
			},
			rowContains: "invalid operation_date",
		},
		{
			name: "transaction value date invalid",
			records: []Record{
				{Type: RecordTypeMeta, Meta: &MetaRecord{Protocol: ProtocolFUI1, Connector: "x", Version: "0.1.0"}},
				{Type: RecordTypeAccount, Account: &AccountRecord{Ref: "a", Name: "A", Currency: "EUR", Kind: AccountKindChecking}},
				{Type: RecordTypeTransaction, Transaction: &TransactionRecord{Account: "a", ID: "t", Date: "2026-11-11", ValueDate: "20261111", Amount: 1, Currency: "EUR", Description: "x", Status: TransactionStatusBooked}},
				{Type: RecordTypeEnd, End: &EndRecord{Transactions: 1}},
			},
			rowContains: "invalid value_date",
		},
		{
			name: "transaction splits need at least two rows",
			records: []Record{
				{Type: RecordTypeMeta, Meta: &MetaRecord{Protocol: ProtocolFUI1, Connector: "x", Version: "0.1.0"}},
				{Type: RecordTypeAccount, Account: &AccountRecord{Ref: "a", Name: "A", Currency: "EUR", Kind: AccountKindChecking}},
				{Type: RecordTypeTransaction, Transaction: &TransactionRecord{
					Account: "a", ID: "t", Date: "2026-11-11", Amount: -100, Currency: "EUR", Description: "x", Status: TransactionStatusBooked,
					Splits: []TransactionSplit{{Category: "Food", Amount: -100}},
				}},
				{Type: RecordTypeEnd, End: &EndRecord{Transactions: 1}},
			},
			rowContains: "at least 2 rows",
		},
		{
			name: "transaction split row requires category",
			records: []Record{
				{Type: RecordTypeMeta, Meta: &MetaRecord{Protocol: ProtocolFUI1, Connector: "x", Version: "0.1.0"}},
				{Type: RecordTypeAccount, Account: &AccountRecord{Ref: "a", Name: "A", Currency: "EUR", Kind: AccountKindChecking}},
				{Type: RecordTypeTransaction, Transaction: &TransactionRecord{
					Account: "a", ID: "t", Date: "2026-11-11", Amount: -100, Currency: "EUR", Description: "x", Status: TransactionStatusBooked,
					Splits: []TransactionSplit{{Category: "", Amount: -40}, {Category: "Food", Amount: -60}},
				}},
				{Type: RecordTypeEnd, End: &EndRecord{Transactions: 1}},
			},
			rowContains: "splits[0].category",
		},
		{
			name: "transaction splits must match amount",
			records: []Record{
				{Type: RecordTypeMeta, Meta: &MetaRecord{Protocol: ProtocolFUI1, Connector: "x", Version: "0.1.0"}},
				{Type: RecordTypeAccount, Account: &AccountRecord{Ref: "a", Name: "A", Currency: "EUR", Kind: AccountKindChecking}},
				{Type: RecordTypeTransaction, Transaction: &TransactionRecord{
					Account: "a", ID: "t", Date: "2026-11-11", Amount: -100, Currency: "EUR", Description: "x", Status: TransactionStatusBooked,
					Splits: []TransactionSplit{{Category: "A", Amount: -40}, {Category: "B", Amount: -50}},
				}},
				{Type: RecordTypeEnd, End: &EndRecord{Transactions: 1}},
			},
			rowContains: "splits sum",
		},
		{
			name: "security_op requires known account",
			records: []Record{
				{Type: RecordTypeMeta, Meta: &MetaRecord{Protocol: ProtocolFUI1, Connector: "x", Version: "0.1.0"}},
				{Type: RecordTypeSecurityOp, SecurityOp: &SecurityOpRecord{
					Account: "missing", Date: "2026-11-11", Kind: SecurityOpKindBuy, Quantity: 1, UnitPrice: 1,
					Security: SecurityMeta{Name: "S", Ticker: "S", Kind: SecurityKindStock, Currency: "EUR", QuantityScale: 0},
				}},
				{Type: RecordTypeEnd, End: &EndRecord{}},
			},
			structural: "security_op references unknown account",
		},
		{
			name: "security_op invalid kind row-level",
			records: []Record{
				{Type: RecordTypeMeta, Meta: &MetaRecord{Protocol: ProtocolFUI1, Connector: "x", Version: "0.1.0"}},
				{Type: RecordTypeAccount, Account: &AccountRecord{Ref: "a", Name: "A", Currency: "EUR", Kind: AccountKindChecking}},
				{Type: RecordTypeSecurityOp, SecurityOp: &SecurityOpRecord{
					Account: "a", Date: "2026-11-11", Kind: SecurityOpKind("bad"), Quantity: 1, UnitPrice: 1,
					Security: SecurityMeta{Name: "S", Ticker: "S", Kind: SecurityKindStock, Currency: "EUR", QuantityScale: 0},
				}},
				{Type: RecordTypeEnd, End: &EndRecord{}},
			},
			rowContains: "invalid security_op kind",
		},
		{
			name: "security_op invalid quantity scale row-level",
			records: []Record{
				{Type: RecordTypeMeta, Meta: &MetaRecord{Protocol: ProtocolFUI1, Connector: "x", Version: "0.1.0"}},
				{Type: RecordTypeAccount, Account: &AccountRecord{Ref: "a", Name: "A", Currency: "EUR", Kind: AccountKindChecking}},
				{Type: RecordTypeSecurityOp, SecurityOp: &SecurityOpRecord{
					Account: "a", Date: "2026-11-11", Kind: SecurityOpKindBuy, Quantity: 1, UnitPrice: 1,
					Security: SecurityMeta{Name: "S", Ticker: "S", Kind: SecurityKindStock, Currency: "EUR", QuantityScale: 9},
				}},
				{Type: RecordTypeEnd, End: &EndRecord{}},
			},
			rowContains: "quantity_scale out of range",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ValidateStream(seqFromRecords(tc.records))
			if tc.structural != "" {
				if err == nil || !strings.Contains(err.Error(), tc.structural) {
					t.Fatalf("error = %v, want structural containing %q", err, tc.structural)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected structural error: %v", err)
			}
			if len(result.RowErrors) != 1 {
				t.Fatalf("row errors = %d, want 1 (%v)", len(result.RowErrors), result.RowErrors)
			}
			if !strings.Contains(result.RowErrors[0].Message, tc.rowContains) {
				t.Fatalf("row error = %q, want substring %q", result.RowErrors[0].Message, tc.rowContains)
			}
		})
	}
}
