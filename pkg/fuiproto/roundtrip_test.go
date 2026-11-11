package fuiproto

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRoundTripEncodeDecode(t *testing.T) {
	records := []Record{
		{
			Type: RecordTypeMeta,
			Meta: &MetaRecord{
				Protocol:  ProtocolFUI1,
				Connector: "fui",
				Version:   "0.1.0",
				Source: SourceMeta{
					File:   "export.fui.jsonl",
					SHA256: "cafebabe",
				},
			},
		},
		{
			Type: RecordTypeAccount,
			Account: &AccountRecord{
				Ref:      "acc-eur",
				Name:     "Main EUR",
				Currency: "EUR",
				Kind:     AccountKindChecking,
				IBAN:     "IT00X0000000000000000000000",
			},
		},
		{
			Type: RecordTypeTransaction,
			Transaction: &TransactionRecord{
				Account:       "acc-eur",
				ID:            "tx-1",
				Date:          "2026-10-03",
				OperationDate: "2026-10-02",
				ValueDate:     "2026-10-03",
				Amount:        -1250,
				Currency:      "EUR",
				Description:   "Groceries",
				Payee:         "Esselunga",
				Status:        TransactionStatusBooked,
				Kind:          TransactionKindPayment,
				Transfer:      "tr-1",
				Original: &OriginalAmount{
					Amount:   -1400,
					Currency: "USD",
				},
				CategoryHint: "groceries",
				Category:     "food",
				Tags:         []string{"home"},
				Notes:        "memo",
				Raw: map[string]any{
					"external_id": "abc",
				},
			},
		},
		{
			Type: RecordTypeBalance,
			Balance: &BalanceRecord{
				Account: "acc-eur",
				Date:    "2026-10-31",
				Amount:  234018,
			},
		},
		{
			Type: RecordTypeAccount,
			Account: &AccountRecord{
				Ref:      "loan-1",
				Name:     "Loan 1",
				Currency: "EUR",
				Kind:     AccountKindLoan,
			},
		},
		{
			Type: RecordTypeSchedule,
			Schedule: &ScheduleRecord{
				Account:   "loan-1",
				ValidFrom: "2026-01-01",
				Rate:      150,
				RateType:  RateTypeFixed,
			},
		},
		{
			Type: RecordTypeInstallment,
			Installment: &InstallmentRecord{
				Account:   "loan-1",
				DueDate:   "2026-11-15",
				Principal: 10000,
				Interest:  2500,
				Fees:      100,
				Remaining: 900000,
			},
		},
		{
			Type: RecordTypeError,
			Error: &ErrorRecord{
				Line:    42,
				Message: "cannot parse source line",
				Raw: map[string]any{
					"csv_line": "bad,data",
				},
			},
		},
		{
			Type: RecordTypeEnd,
			End: &EndRecord{
				Transactions: 1,
				Errors:       1,
			},
		},
	}

	var buf bytes.Buffer
	if err := EncodeSliceJSONL(&buf, records); err != nil {
		t.Fatalf("encode: %v", err)
	}

	var decoded []Record
	for record, err := range DecodeJSONL(bytes.NewReader(buf.Bytes())) {
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		decoded = append(decoded, record)
	}

	if diff := cmp.Diff(records, decoded); diff != "" {
		t.Fatalf("round-trip mismatch (-want +got):\n%s", diff)
	}
}

func TestSpecExampleDecodesAndValidates(t *testing.T) {
	example := strings.Join([]string{
		`{"type":"meta","protocol":"fui/1","connector":"revolut","version":"0.1.0","source":{"file":"revolut-2026-10.csv","sha256":"9f2c"}}`,
		`{"type":"account","ref":"revolut-EUR","name":"Revolut EUR","currency":"EUR","kind":"checking"}`,
		`{"type":"account","ref":"revolut-USD","name":"Revolut USD","currency":"USD","kind":"checking"}`,
		`{"type":"transaction","account":"revolut-EUR","id":"rv-8c1e","date":"2026-10-03","amount":-1250,"currency":"EUR","description":"Esselunga Milano","status":"booked","kind":"payment"}`,
		`{"type":"transaction","account":"revolut-EUR","id":"rv-a41b","date":"2026-10-05","amount":-10000,"currency":"EUR","description":"Exchanged to USD","status":"booked","kind":"exchange","transfer":"fx-20261005-1"}`,
		`{"type":"transaction","account":"revolut-USD","id":"rv-a41c","date":"2026-10-05","amount":10842,"currency":"USD","description":"Exchanged from EUR","status":"booked","kind":"exchange","transfer":"fx-20261005-1"}`,
		`{"type":"balance","account":"revolut-EUR","date":"2026-10-31","amount":234018}`,
		`{"type":"end","transactions":3,"errors":0}`,
	}, "\n") + "\n"

	result, err := ValidateStream(DecodeJSONL(strings.NewReader(example)))
	if err != nil {
		t.Fatalf("validate example stream: %v", err)
	}
	if len(result.RowErrors) != 0 {
		t.Fatalf("example stream has row errors: %v", result.RowErrors)
	}
	if result.End.Transactions != 3 || result.End.Errors != 0 {
		t.Fatalf("unexpected end counts: %+v", result.End)
	}
}
