package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/0xballo/fui/pkg/fuiproto"
)

type revolutResult struct {
	accounts     []fuiproto.AccountRecord
	transactions []fuiproto.Record
	balances     []fuiproto.Record
	errors       []fuiproto.ErrorRecord
}

type revolutRow struct {
	line          int
	accountRef    string
	accountName   string
	accountKind   fuiproto.AccountKind
	currency      string
	date          string
	operationDate string
	amount        int64
	fee           int64
	balance       *int64
	description   string
	payee         string
	status        fuiproto.TransactionStatus
	kind          fuiproto.TransactionKind
	id            string
	sortAt        time.Time
	raw           map[string]any
}

func parseRevolutCSV(r io.Reader, since *string) (revolutResult, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return revolutResult{}, fmt.Errorf("read header: %w", err)
	}

	columns, err := revolutColumns(header)
	if err != nil {
		return revolutResult{}, err
	}

	rows, parseErrors, err := readRevolutRows(reader, columns)
	if err != nil {
		return revolutResult{}, err
	}

	sortRows(rows)
	return materializeRows(rows, parseErrors, since), nil
}

func revolutColumns(header []string) (map[string]int, error) {
	out := make(map[string]int, len(header))
	for i, name := range header {
		out[strings.ToLower(strings.TrimSpace(name))] = i
	}

	required := []string{
		"type",
		"product",
		"started date",
		"completed date",
		"description",
		"amount",
		"fee",
		"currency",
		"state",
		"balance",
	}

	for _, name := range required {
		if _, ok := out[name]; !ok {
			return nil, fmt.Errorf("missing required column %q", name)
		}
	}
	return out, nil
}

func readRevolutRows(reader *csv.Reader, columns map[string]int) ([]revolutRow, []fuiproto.ErrorRecord, error) {
	line := 1
	rows := make([]revolutRow, 0)
	parseErrors := make([]fuiproto.ErrorRecord, 0)

	for {
		record, err := reader.Read()
		if err == io.EOF {
			return rows, parseErrors, nil
		}
		line++
		if err != nil {
			parseErrors = append(parseErrors, revolutError(line, fmt.Sprintf("read CSV row: %v", err), nil))
			continue
		}
		if isBlankRow(record) {
			continue
		}

		row, include, rowErr := parseRevolutRow(record, line, columns)
		if rowErr != nil {
			parseErrors = append(parseErrors, *rowErr)
			continue
		}
		if include {
			rows = append(rows, row)
		}
	}
}

func parseRevolutRow(record []string, line int, columns map[string]int) (revolutRow, bool, *fuiproto.ErrorRecord) {
	raw := revolutRawRow(record, columns)

	state, include, rowErr := parseRevolutState(line, raw)
	if rowErr != nil {
		return revolutRow{}, false, rowErr
	}
	if !include {
		return revolutRow{}, false, nil
	}

	date, sortAt, err := revolutDates(raw["completed date"], raw["started date"], state == "PENDING")
	if err != nil {
		rowErr := revolutError(line, err.Error(), raw)
		return revolutRow{}, false, &rowErr
	}

	amount, fee, balance, rowErr := parseRevolutAmounts(line, raw)
	if rowErr != nil {
		return revolutRow{}, false, rowErr
	}

	product := slug(raw["product"])
	currency := strings.ToUpper(raw["currency"])
	description := normalizeSpaces(raw["description"])

	return revolutRow{
		line:          line,
		accountRef:    "revolut-" + product + "-" + currency,
		accountName:   "Revolut " + titleWords(product) + " " + currency,
		accountKind:   revolutAccountKind(raw["product"]),
		currency:      currency,
		date:          date,
		operationDate: parseOptionalDate(raw["started date"]),
		amount:        amount,
		fee:           fee,
		balance:       balance,
		description:   description,
		payee:         extractRevolutPayee(raw["type"], description),
		status:        revolutStatus(state),
		kind:          revolutKind(raw["type"]),
		id: hashValues(
			raw["started date"],
			raw["completed date"],
			raw["amount"],
			description,
			raw["balance"],
		),
		sortAt: sortAt,
		raw:    toAnyMap(raw),
	}, true, nil
}

func parseRevolutState(line int, raw map[string]string) (string, bool, *fuiproto.ErrorRecord) {
	state := strings.ToUpper(raw["state"])
	switch state {
	case "REVERTED", "DECLINED":
		return "", false, nil
	case "COMPLETED", "PENDING":
		return state, true, nil
	default:
		err := revolutError(line, fmt.Sprintf("unsupported state %q", raw["state"]), raw)
		return "", false, &err
	}
}

func parseRevolutAmounts(
	line int,
	raw map[string]string,
) (int64, int64, *int64, *fuiproto.ErrorRecord) {
	amount, err := parseMinorUnits(raw["amount"])
	if err != nil {
		rowErr := revolutError(line, fmt.Sprintf("invalid amount: %v", err), raw)
		return 0, 0, nil, &rowErr
	}

	fee := int64(0)
	parsedFee, err := parseOptionalMinorUnits(raw["fee"])
	if err != nil {
		rowErr := revolutError(line, fmt.Sprintf("invalid fee: %v", err), raw)
		return 0, 0, nil, &rowErr
	}
	if parsedFee != nil {
		fee = absMinorUnits(*parsedFee)
	}

	balance, err := parseOptionalMinorUnits(raw["balance"])
	if err != nil {
		rowErr := revolutError(line, fmt.Sprintf("invalid balance: %v", err), raw)
		return 0, 0, nil, &rowErr
	}

	return amount, fee, balance, nil
}

func revolutDates(completedDate, startedDate string, pending bool) (string, time.Time, error) {
	completedDate = strings.TrimSpace(completedDate)
	startedDate = strings.TrimSpace(startedDate)

	if completedDate != "" {
		date := parseOptionalDate(completedDate)
		if date == "" {
			return "", time.Time{}, fmt.Errorf("invalid completed date %q", completedDate)
		}

		sortAt, err := parseTime(completedDate)
		if err != nil {
			sortAt = mustDate(date)
		}
		return date, sortAt, nil
	}

	if pending && startedDate != "" {
		date := parseOptionalDate(startedDate)
		if date == "" {
			return "", time.Time{}, fmt.Errorf("invalid started date %q", startedDate)
		}

		sortAt, err := parseTime(startedDate)
		if err != nil {
			sortAt = mustDate(date)
		}
		return date, sortAt, nil
	}

	return "", time.Time{}, fmt.Errorf("missing completed date")
}

func materializeRows(rows []revolutRow, parseErrors []fuiproto.ErrorRecord, since *string) revolutResult {
	accountMap := map[string]fuiproto.AccountRecord{}
	transactions := make([]fuiproto.Record, 0)
	balances := map[string]fuiproto.BalanceRecord{}

	for _, row := range rows {
		if since != nil && row.date < *since {
			continue
		}

		accountMap[row.accountRef] = fuiproto.AccountRecord{
			Ref:      row.accountRef,
			Name:     row.accountName,
			Currency: row.currency,
			Kind:     row.accountKind,
		}

		transactions = append(transactions, makeMainTransaction(row))
		if row.fee != 0 {
			transactions = append(transactions, makeFeeTransaction(row))
		}

		if row.balance != nil {
			balances[row.accountRef] = fuiproto.BalanceRecord{
				Account: row.accountRef,
				Date:    row.date,
				Amount:  *row.balance,
			}
		}
	}

	return revolutResult{
		accounts:     sortAccounts(accountMap),
		transactions: transactions,
		balances:     sortBalances(balances),
		errors:       parseErrors,
	}
}

func makeMainTransaction(row revolutRow) fuiproto.Record {
	var balanceAfter *int64
	if row.balance != nil {
		value := *row.balance
		if row.fee != 0 {
			value = value + row.fee
		}
		balanceAfter = &value
	}

	return fuiproto.Record{
		Type: fuiproto.RecordTypeTransaction,
		Transaction: &fuiproto.TransactionRecord{
			Account:       row.accountRef,
			ID:            row.id,
			Date:          row.date,
			OperationDate: row.operationDate,
			Amount:        row.amount,
			Currency:      row.currency,
			Description:   row.description,
			Payee:         row.payee,
			Status:        row.status,
			Kind:          row.kind,
			BalanceAfter:  balanceAfter,
			Raw:           row.raw,
		},
	}
}

func makeFeeTransaction(row revolutRow) fuiproto.Record {
	var balanceAfter *int64
	if row.balance != nil {
		value := *row.balance
		balanceAfter = &value
	}

	return fuiproto.Record{
		Type: fuiproto.RecordTypeTransaction,
		Transaction: &fuiproto.TransactionRecord{
			Account:       row.accountRef,
			ID:            row.id + "-fee",
			Date:          row.date,
			OperationDate: row.operationDate,
			Amount:        -row.fee,
			Currency:      row.currency,
			Description:   "Fee: " + row.description,
			Status:        row.status,
			Kind:          fuiproto.TransactionKindFee,
			BalanceAfter:  balanceAfter,
			Raw:           row.raw,
		},
	}
}

func sortRows(rows []revolutRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].sortAt.Equal(rows[j].sortAt) {
			return rows[i].line < rows[j].line
		}
		return rows[i].sortAt.Before(rows[j].sortAt)
	})
}

func sortAccounts(accounts map[string]fuiproto.AccountRecord) []fuiproto.AccountRecord {
	keys := make([]string, 0, len(accounts))
	for key := range accounts {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]fuiproto.AccountRecord, 0, len(keys))
	for _, key := range keys {
		out = append(out, accounts[key])
	}
	return out
}

func sortBalances(balances map[string]fuiproto.BalanceRecord) []fuiproto.Record {
	keys := make([]string, 0, len(balances))
	for key := range balances {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]fuiproto.Record, 0, len(keys))
	for _, key := range keys {
		record := balances[key]
		out = append(out, fuiproto.Record{Type: fuiproto.RecordTypeBalance, Balance: &record})
	}
	return out
}

func emitStream(w io.Writer, sourcePath string, source []byte, result revolutResult) error {
	encoder := fuiproto.NewJSONLEncoder(w)
	transactionsCount := len(result.transactions)

	if err := encoder.Encode(metaRecord(sourcePath, source)); err != nil {
		return err
	}
	for _, account := range result.accounts {
		record := fuiproto.Record{Type: fuiproto.RecordTypeAccount, Account: &account}
		if err := encoder.Encode(record); err != nil {
			return err
		}
	}
	for _, record := range result.transactions {
		if err := encoder.Encode(record); err != nil {
			return err
		}
	}
	for _, errorRow := range result.errors {
		record := fuiproto.Record{Type: fuiproto.RecordTypeError, Error: &errorRow}
		if err := encoder.Encode(record); err != nil {
			return err
		}
	}
	for _, record := range result.balances {
		if err := encoder.Encode(record); err != nil {
			return err
		}
	}

	return encoder.Encode(fuiproto.Record{
		Type: fuiproto.RecordTypeEnd,
		End: &fuiproto.EndRecord{
			Transactions: transactionsCount,
			Errors:       len(result.errors),
		},
	})
}

func metaRecord(sourcePath string, source []byte) fuiproto.Record {
	return fuiproto.Record{
		Type: fuiproto.RecordTypeMeta,
		Meta: &fuiproto.MetaRecord{
			Protocol:  fuiproto.ProtocolFUI1,
			Connector: connectorName,
			Version:   connectorVersion,
			Source: fuiproto.SourceMeta{
				File:   filepath.Base(sourcePath),
				SHA256: sha256Hex(source),
			},
		},
	}
}

func revolutStatus(state string) fuiproto.TransactionStatus {
	if state == "PENDING" {
		return fuiproto.TransactionStatusPending
	}
	return fuiproto.TransactionStatusBooked
}

func revolutKind(rawType string) fuiproto.TransactionKind {
	switch strings.ToLower(strings.TrimSpace(rawType)) {
	case "card payment":
		return fuiproto.TransactionKindPayment
	case "transfer":
		return fuiproto.TransactionKindTransfer
	case "card refund":
		return fuiproto.TransactionKindRefund
	case "exchange":
		return fuiproto.TransactionKindExchange
	case "topup":
		return fuiproto.TransactionKindTransfer
	default:
		return fuiproto.TransactionKindOther
	}
}

func revolutAccountKind(product string) fuiproto.AccountKind {
	if strings.EqualFold(strings.TrimSpace(product), "savings") {
		return fuiproto.AccountKindSavings
	}
	return fuiproto.AccountKindChecking
}

func extractRevolutPayee(rawType, description string) string {
	loweredType := strings.ToLower(strings.TrimSpace(rawType))
	switch loweredType {
	case "card payment":
		return description
	case "transfer":
		return stripPrefix(description, "Transfer from ", "Transfer to ", "From ", "To ")
	default:
		return ""
	}
}

func stripPrefix(value string, prefixes ...string) string {
	for _, prefix := range prefixes {
		if strings.HasPrefix(strings.ToLower(value), strings.ToLower(prefix)) {
			return strings.TrimSpace(value[len(prefix):])
		}
	}
	return ""
}

func parseMinorUnits(value string) (int64, error) {
	return parseMinorUnitsWithSeparator(value, '.')
}

func parseOptionalMinorUnits(value string) (*int64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}

	parsed, err := parseMinorUnits(trimmed)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseMinorUnitsWithSeparator(value string, decimalSeparator rune) (int64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("empty value")
	}

	sign := int64(1)
	if strings.HasPrefix(trimmed, "-") {
		sign = -1
		trimmed = strings.TrimSpace(trimmed[1:])
	} else if strings.HasPrefix(trimmed, "+") {
		trimmed = strings.TrimSpace(trimmed[1:])
	}
	if trimmed == "" {
		return 0, fmt.Errorf("missing digits")
	}

	parts := strings.Split(trimmed, string(decimalSeparator))
	if len(parts) > 2 {
		return 0, fmt.Errorf("invalid decimal %q", value)
	}
	if parts[0] == "" {
		parts[0] = "0"
	}
	if !digitsOnly(parts[0]) {
		return 0, fmt.Errorf("invalid integer part %q", parts[0])
	}

	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse integer part: %w", err)
	}

	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if fraction != "" && !digitsOnly(fraction) {
		return 0, fmt.Errorf("invalid fractional part %q", fraction)
	}

	minor, err := scaleFraction(whole, fraction)
	if err != nil {
		return 0, err
	}
	if sign < 0 {
		minor = -minor
	}
	return minor, nil
}

func scaleFraction(whole int64, fraction string) (int64, error) {
	if whole > (1<<63-1)/100 {
		return 0, fmt.Errorf("amount overflow")
	}
	base := whole * 100

	if len(fraction) == 0 {
		return base, nil
	}
	if len(fraction) == 1 {
		fraction += "0"
	}

	firstTwo := fraction
	if len(firstTwo) > 2 {
		firstTwo = firstTwo[:2]
	}
	cents, err := strconv.ParseInt(firstTwo, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse fraction: %w", err)
	}

	if len(fraction) > 2 && fraction[2] >= '5' {
		cents++
	}
	if cents >= 100 {
		base += 100
		cents -= 100
	}
	return base + cents, nil
}

func parseTime(value string) (time.Time, error) {
	layouts := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, strings.TrimSpace(value))
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported datetime %q", value)
}

func parseOptionalDate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < len("2006-01-02") {
		return ""
	}
	date := value[:10]
	if err := fuiproto.ValidateDate(date); err != nil {
		return ""
	}
	return date
}

func mustDate(value string) time.Time {
	parsed, _ := time.Parse("2006-01-02", value)
	return parsed
}

func isBlankRow(record []string) bool {
	for _, field := range record {
		if strings.TrimSpace(field) != "" {
			return false
		}
	}
	return true
}

func revolutRawRow(record []string, columns map[string]int) map[string]string {
	out := map[string]string{}
	for name, index := range columns {
		out[name] = field(record, index)
	}
	return out
}

func field(record []string, index int) string {
	if index < 0 || index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}

func revolutError(line int, message string, raw map[string]string) fuiproto.ErrorRecord {
	record := fuiproto.ErrorRecord{
		Line:    line,
		Message: message,
	}
	if raw != nil {
		record.Raw = toAnyMap(raw)
	}
	return record
}

func toAnyMap(in map[string]string) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func hashValues(values ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(values, "|")))
	return hex.EncodeToString(sum[:])
}

func sha256Hex(source []byte) string {
	sum := sha256.Sum256(source)
	return hex.EncodeToString(sum[:])
}

func normalizeSpaces(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func titleWords(value string) string {
	parts := strings.Split(value, "-")
	for i, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}

func slug(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "unknown"
	}
	return result
}

func digitsOnly(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func absMinorUnits(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
