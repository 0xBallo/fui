package fuiproto

import (
	"iter"
)

func validStreamRecords() []Record {
	return []Record{
		{
			Type: RecordTypeMeta,
			Meta: &MetaRecord{
				Protocol:  ProtocolFUI1,
				Connector: "test",
				Version:   "0.1.0",
				Source: SourceMeta{
					File:   "input.csv",
					SHA256: "abc",
				},
			},
		},
		{
			Type: RecordTypeAccount,
			Account: &AccountRecord{
				Ref:      "acc-1",
				Name:     "Account 1",
				Currency: "EUR",
				Kind:     AccountKindChecking,
			},
		},
		{
			Type: RecordTypeTransaction,
			Transaction: &TransactionRecord{
				Account:     "acc-1",
				ID:          "tx-1",
				Date:        "2026-10-01",
				Amount:      -1234,
				Currency:    "EUR",
				Description: "Coffee",
				Status:      TransactionStatusBooked,
				Kind:        TransactionKindPayment,
				Tags:        []string{"food"},
				Raw: map[string]any{
					"source_id": "x1",
				},
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
}

func seqFromRecords(records []Record) iter.Seq2[Record, error] {
	return func(yield func(Record, error) bool) {
		for _, record := range records {
			if !yield(record, nil) {
				return
			}
		}
	}
}
