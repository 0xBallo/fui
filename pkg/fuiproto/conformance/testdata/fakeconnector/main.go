package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/0xballo/fui/pkg/fuiproto"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "missing command")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "describe":
		describe()
	case "fetch":
		fetch()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		os.Exit(1)
	}
}

func describe() {
	response := fuiproto.DescribeResponse{
		Protocol: fuiproto.ProtocolFUI1,
		Name:     "fake",
		Version:  "0.1.0",
		AcceptedFileTypes: []string{
			".csv",
		},
		ConfigParams: []fuiproto.ConfigParam{
			{
				Key:         "token",
				Type:        "string",
				Required:    true,
				Description: "Fake access token",
			},
		},
	}

	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		fmt.Fprintf(os.Stderr, "encode describe: %v\n", err)
		os.Exit(1)
	}
}

func fetch() {
	var request fuiproto.FetchRequest
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		fmt.Fprintf(os.Stderr, "decode request: %v\n", err)
		os.Exit(fuiproto.ExitCodeBadConfig)
	}

	if request.Protocol != fuiproto.ProtocolFUI1 {
		fmt.Fprintln(os.Stderr, "unsupported protocol")
		os.Exit(fuiproto.ExitCodeBadConfig)
	}

	token, _ := request.Config["token"].(string)
	if token == "" {
		fmt.Fprintln(os.Stderr, "missing token")
		os.Exit(fuiproto.ExitCodeBadConfig)
	}

	if request.File == nil || *request.File == "" {
		fmt.Fprintln(os.Stderr, "missing source file")
		os.Exit(fuiproto.ExitCodeUnreadableSource)
	}

	content, err := os.ReadFile(*request.File)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read source file: %v\n", err)
		os.Exit(fuiproto.ExitCodeUnreadableSource)
	}

	if err := emitRecords(*request.File, content); err != nil {
		fmt.Fprintf(os.Stderr, "emit records: %v\n", err)
		os.Exit(1)
	}
}

func emitRecords(path string, content []byte) error {
	hash := sha256.Sum256(content)
	sourceHash := hex.EncodeToString(hash[:])
	enc := fuiproto.NewJSONLEncoder(os.Stdout)

	if err := enc.Encode(fuiproto.Record{
		Type: fuiproto.RecordTypeMeta,
		Meta: &fuiproto.MetaRecord{
			Protocol:  fuiproto.ProtocolFUI1,
			Connector: "fake",
			Version:   "0.1.0",
			Source: fuiproto.SourceMeta{
				File:   filepath.Base(path),
				SHA256: sourceHash,
			},
		},
	}); err != nil {
		return err
	}

	if err := enc.Encode(fuiproto.Record{
		Type: fuiproto.RecordTypeAccount,
		Account: &fuiproto.AccountRecord{
			Ref:      "fake-EUR",
			Name:     "Fake EUR",
			Currency: "EUR",
			Kind:     fuiproto.AccountKindChecking,
		},
	}); err != nil {
		return err
	}

	transactions := 0
	for lineIndex, row := range parseRows(content) {
		date, amount, description, rowErr := parseRow(row)
		if rowErr != nil {
			if err := enc.Encode(fuiproto.Record{
				Type: fuiproto.RecordTypeError,
				Error: &fuiproto.ErrorRecord{
					Line:    lineIndex + 1,
					Message: rowErr.Error(),
					Raw: map[string]any{
						"row": row,
					},
				},
			}); err != nil {
				return err
			}
			continue
		}

		transactions++
		tx := &fuiproto.TransactionRecord{
			Account:     "fake-EUR",
			ID:          fmt.Sprintf("fake-%03d", lineIndex+1),
			Date:        date,
			Amount:      amount,
			Currency:    "EUR",
			Description: description,
			Status:      fuiproto.TransactionStatusBooked,
			Kind:        fuiproto.TransactionKindPayment,
		}
		if amount < 0 {
			half := amount / 2
			tx.Splits = []fuiproto.TransactionSplit{
				{Category: "Shopping", Amount: half, Note: "part 1"},
				{Category: "Food", Amount: amount - half, Note: "part 2"},
			}
		}
		if err := enc.Encode(fuiproto.Record{
			Type:        fuiproto.RecordTypeTransaction,
			Transaction: tx,
		}); err != nil {
			return err
		}
	}

	return enc.Encode(fuiproto.Record{
		Type: fuiproto.RecordTypeEnd,
		End: &fuiproto.EndRecord{
			Transactions: transactions,
			Errors:       0,
		},
	})
}

func parseRows(content []byte) []string {
	rows := make([]string, 0)
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		row := strings.TrimSpace(scanner.Text())
		if row == "" {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

func parseRow(row string) (string, int64, string, error) {
	parts := strings.SplitN(row, ",", 3)
	if len(parts) != 3 {
		return "", 0, "", fmt.Errorf("expected 3 columns date,amount,description")
	}

	amount, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil {
		return "", 0, "", fmt.Errorf("invalid amount %q: %w", parts[1], err)
	}

	return strings.TrimSpace(parts[0]), amount, strings.TrimSpace(parts[2]), nil
}
