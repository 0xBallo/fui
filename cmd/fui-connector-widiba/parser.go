package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/xuri/excelize/v2"

	"github.com/0xballo/fui/pkg/fuiproto"
)

const widibaSheetName = "Movimenti conto"

var (
	ibanRegex = regexp.MustCompile(`(?i)\b[A-Z]{2}\d{2}[A-Z0-9]{10,30}\b`)

	bonIstRegex  = regexp.MustCompile(`(?i)\bBon\.\s*Ist\.\s*([A-Z0-9\-]+)`)
	bonSepaRegex = regexp.MustCompile(`(?i)\bBon\.\s*Sepa\.?\s*([A-Z0-9\-]+)`)
	sddRegex     = regexp.MustCompile(`(?i)\bAddebito\s*Sdd\s*N\.\s*([A-Z0-9\-]+)`)

	payeeAFavoreRegex = regexp.MustCompile(`(?i)A\s*Favore\s*[:\-]?\s*([^\n;]+)`)
	payeeOrdRegex     = regexp.MustCompile(`(?i)\bOrd:\s*([^\n;]+)`)
)

type widibaSummary struct {
	iban    string
	opening int64
	closing int64
}

type widibaMovement struct {
	line             int
	date             string
	valueDate        string
	causale          string
	description      string
	amount           int64
	payee            string
	counterpartyIBAN string
	kind             fuiproto.TransactionKind
	id               string
	raw              map[string]any
}

type widibaResult struct {
	accounts     []fuiproto.AccountRecord
	transactions []fuiproto.Record
	balance      fuiproto.Record
	errors       []fuiproto.ErrorRecord
}

func parseWidibaWorkbook(content []byte, since *string) (widibaResult, error) {
	workbook, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		return widibaResult{}, fmt.Errorf("open workbook: %w", err)
	}
	defer func() {
		_ = workbook.Close()
	}()

	rows, err := workbook.GetRows(widibaSheetName)
	if err != nil {
		return widibaResult{}, fmt.Errorf("read sheet %q: %w", widibaSheetName, err)
	}

	headerLine, columns, err := findMovementHeader(rows)
	if err != nil {
		return widibaResult{}, err
	}

	summary, err := parseSummary(rows[:headerLine-1])
	if err != nil {
		return widibaResult{}, err
	}

	movements, parseErrors := parseMovements(rows, headerLine, columns)
	return materializeWidiba(summary, movements, parseErrors, since), nil
}

func findMovementHeader(rows [][]string) (int, map[string]int, error) {
	for i, row := range rows {
		columns := headerColumns(row)
		if hasRequiredColumns(columns) {
			return i + 1, columns, nil
		}
	}
	return 0, nil, fmt.Errorf("cannot find movement header row containing data cont")
}

func headerColumns(row []string) map[string]int {
	columns := make(map[string]int, len(row))
	for idx, cell := range row {
		key := normalizeHeader(cell)
		if key != "" {
			columns[key] = idx
		}
	}
	return columns
}

func hasRequiredColumns(columns map[string]int) bool {
	required := []string{"DATACONT", "DATAVAL", "CAUSALE", "DESCRIZIONE", "IMPORTO"}
	for _, key := range required {
		if _, ok := columns[key]; !ok {
			return false
		}
	}
	return true
}

func parseSummary(rows [][]string) (widibaSummary, error) {
	summary := widibaSummary{}
	foundIBAN := false
	foundOpening := false
	foundClosing := false

	for _, row := range rows {
		for col, cell := range row {
			label := normalizeSpaces(cell)
			iban := extractFirstIBAN(label)
			if iban != "" && !foundIBAN {
				summary.iban = iban
				foundIBAN = true
			}

			lowered := strings.ToLower(label)
			switch {
			case strings.Contains(lowered, "saldo iniziale"):
				amount, ok := firstAmountFromCells(row[col+1:])
				if ok {
					summary.opening = amount
					foundOpening = true
				}
			case strings.Contains(lowered, "saldo finale"):
				amount, ok := firstAmountFromCells(row[col+1:])
				if ok {
					summary.closing = amount
					foundClosing = true
				}
			}
		}
	}

	if !foundIBAN {
		return widibaSummary{}, fmt.Errorf("cannot extract IBAN from summary")
	}
	if !foundOpening || !foundClosing {
		return widibaSummary{}, fmt.Errorf("cannot extract opening/final balances from summary")
	}
	return summary, nil
}

func firstAmountFromCells(cells []string) (int64, bool) {
	for _, cell := range cells {
		amount, err := parseEuroMinorUnits(cell)
		if err == nil {
			return amount, true
		}
	}
	return 0, false
}

func parseMovements(rows [][]string, headerLine int, columns map[string]int) ([]widibaMovement, []fuiproto.ErrorRecord) {
	movements := make([]widibaMovement, 0)
	errors := make([]fuiproto.ErrorRecord, 0)
	fallbackIDIndex := map[string]int{}

	for rowIndex := headerLine; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex]
		line := rowIndex + 1

		if isTotalRow(row) {
			break
		}
		if rowIsBlank(row) {
			continue
		}

		movement, rowErr := parseMovementRow(row, line, columns, fallbackIDIndex)
		if rowErr != nil {
			errors = append(errors, *rowErr)
			continue
		}
		movements = append(movements, movement)
	}

	return movements, errors
}

func parseMovementRow(
	row []string,
	line int,
	columns map[string]int,
	fallbackIDIndex map[string]int,
) (widibaMovement, *fuiproto.ErrorRecord) {
	raw := map[string]string{
		"date":        field(row, columns["DATACONT"]),
		"value_date":  field(row, columns["DATAVAL"]),
		"causale":     field(row, columns["CAUSALE"]),
		"description": field(row, columns["DESCRIZIONE"]),
		"amount":      field(row, columns["IMPORTO"]),
	}

	date, err := parseWidibaDate(raw["date"])
	if err != nil {
		rowErr := widibaError(line, fmt.Sprintf("invalid booking date: %v", err), raw)
		return widibaMovement{}, &rowErr
	}

	valueDate := ""
	if strings.TrimSpace(raw["value_date"]) != "" {
		valueDate, err = parseWidibaDate(raw["value_date"])
		if err != nil {
			rowErr := widibaError(line, fmt.Sprintf("invalid value date: %v", err), raw)
			return widibaMovement{}, &rowErr
		}
	}

	amount, err := parseEuroMinorUnits(raw["amount"])
	if err != nil {
		rowErr := widibaError(line, fmt.Sprintf("invalid amount: %v", err), raw)
		return widibaMovement{}, &rowErr
	}

	causale := normalizeSpaces(raw["causale"])
	description := normalizeSpaces(raw["description"])
	id := widibaMovementID(date, causale, description, amount, fallbackIDIndex)

	return widibaMovement{
		line:             line,
		date:             date,
		valueDate:        valueDate,
		causale:          causale,
		description:      description,
		amount:           amount,
		payee:            extractPayee(description),
		counterpartyIBAN: extractFirstIBAN(description),
		kind:             widibaKind(causale),
		id:               id,
		raw:              toAnyMap(raw),
	}, nil
}

func materializeWidiba(
	summary widibaSummary,
	movements []widibaMovement,
	parseErrors []fuiproto.ErrorRecord,
	since *string,
) widibaResult {
	mainRef := "widiba-" + accountSuffix(summary.iban)
	errors := append([]fuiproto.ErrorRecord{}, parseErrors...)
	transactions, lineaLiberaUsed, latestDate, sumAmounts := materializeMovementRecords(mainRef, movements, since)
	errors = append(errors, summaryMismatchError(summary, sumAmounts)...)
	accounts := widibaAccounts(summary, lineaLiberaUsed)

	if latestDate == "" {
		latestDate = fallbackLatestDate(movements)
	}

	return widibaResult{
		accounts:     accounts,
		transactions: transactions,
		balance: fuiproto.Record{
			Type: fuiproto.RecordTypeBalance,
			Balance: &fuiproto.BalanceRecord{
				Account: mainRef,
				Date:    latestDate,
				Amount:  summary.closing,
			},
		},
		errors: errors,
	}
}

func materializeMovementRecords(
	mainRef string,
	movements []widibaMovement,
	since *string,
) ([]fuiproto.Record, bool, string, int64) {
	transactions := make([]fuiproto.Record, 0)
	lineaLiberaUsed := false
	latestDate := ""
	sumAmounts := int64(0)

	for _, movement := range movements {
		sumAmounts += movement.amount
		if since != nil && movement.date < *since {
			continue
		}

		transactions = append(transactions, widibaTransaction(mainRef, movement, ""))
		latestDate = maxDate(latestDate, movement.date)

		if isLineaLibera(movement.causale) {
			lineaLiberaUsed = true
			transferID := "linea-libera-" + movement.id
			transactions[len(transactions)-1].Transaction.Transfer = transferID
			transactions = append(transactions, widibaTransferCounterpart(movement, transferID))
		}
	}

	return transactions, lineaLiberaUsed, latestDate, sumAmounts
}

func summaryMismatchError(summary widibaSummary, sumAmounts int64) []fuiproto.ErrorRecord {
	expectedClosing := summary.opening + sumAmounts
	if expectedClosing == summary.closing {
		return nil
	}

	return []fuiproto.ErrorRecord{
		{
			Line:    1,
			Message: fmt.Sprintf("summary balance mismatch: opening + movements = %d, closing = %d", expectedClosing, summary.closing),
			Raw: map[string]any{
				"opening": summary.opening,
				"closing": summary.closing,
			},
		},
	}
}

func widibaAccounts(summary widibaSummary, lineaLiberaUsed bool) []fuiproto.AccountRecord {
	mainAccount := fuiproto.AccountRecord{
		Ref:      "widiba-" + accountSuffix(summary.iban),
		Name:     "Widiba " + accountSuffix(summary.iban),
		Currency: "EUR",
		Kind:     fuiproto.AccountKindChecking,
		IBAN:     summary.iban,
	}

	accounts := []fuiproto.AccountRecord{mainAccount}
	if lineaLiberaUsed {
		accounts = append(accounts, fuiproto.AccountRecord{
			Ref:      "widiba-linea-libera",
			Name:     "Widiba Linea Libera",
			Currency: "EUR",
			Kind:     fuiproto.AccountKindSavings,
		})
	}
	return accounts
}

func widibaTransaction(accountRef string, movement widibaMovement, transfer string) fuiproto.Record {
	return fuiproto.Record{
		Type: fuiproto.RecordTypeTransaction,
		Transaction: &fuiproto.TransactionRecord{
			Account:          accountRef,
			ID:               movement.id,
			Date:             movement.date,
			ValueDate:        movement.valueDate,
			Amount:           movement.amount,
			Currency:         "EUR",
			Description:      movement.description,
			Payee:            movement.payee,
			CounterpartyIBAN: movement.counterpartyIBAN,
			Status:           fuiproto.TransactionStatusBooked,
			Kind:             movement.kind,
			Transfer:         transfer,
			Raw:              movement.raw,
		},
	}
}

func widibaTransferCounterpart(movement widibaMovement, transferID string) fuiproto.Record {
	return fuiproto.Record{
		Type: fuiproto.RecordTypeTransaction,
		Transaction: &fuiproto.TransactionRecord{
			Account:     "widiba-linea-libera",
			ID:          movement.id + "-linea-libera",
			Date:        movement.date,
			ValueDate:   movement.valueDate,
			Amount:      -movement.amount,
			Currency:    "EUR",
			Description: movement.description,
			Status:      fuiproto.TransactionStatusBooked,
			Kind:        fuiproto.TransactionKindTransfer,
			Transfer:    transferID,
			Raw:         movement.raw,
		},
	}
}

func emitStream(w io.Writer, sourcePath string, source []byte, result widibaResult) error {
	encoder := fuiproto.NewJSONLEncoder(w)
	if err := encoder.Encode(metaRecord(sourcePath, source)); err != nil {
		return err
	}

	sort.Slice(result.accounts, func(i, j int) bool {
		return result.accounts[i].Ref < result.accounts[j].Ref
	})
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
	for _, rowErr := range result.errors {
		record := fuiproto.Record{Type: fuiproto.RecordTypeError, Error: &rowErr}
		if err := encoder.Encode(record); err != nil {
			return err
		}
	}
	if err := encoder.Encode(result.balance); err != nil {
		return err
	}

	return encoder.Encode(fuiproto.Record{
		Type: fuiproto.RecordTypeEnd,
		End: &fuiproto.EndRecord{
			Transactions: len(result.transactions),
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

func parseWidibaDate(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("empty value")
	}

	layouts := []string{
		"2006-01-02",
		"02/01/2006",
		"2/1/2006",
		"02-01-2006",
		"2-1-2006",
		"02.01.2006",
		"2.1.2006",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return parsed.Format("2006-01-02"), nil
		}
	}

	serial, err := parseSerialInt(trimmed)
	if err != nil {
		return "", fmt.Errorf("unsupported date %q", value)
	}
	return excelSerialToDate(serial)
}

func parseSerialInt(value string) (int64, error) {
	sanitized := strings.ReplaceAll(value, ",", ".")
	if idx := strings.IndexByte(sanitized, '.'); idx >= 0 {
		sanitized = sanitized[:idx]
	}
	sanitized = strings.TrimSpace(sanitized)
	if sanitized == "" {
		return 0, fmt.Errorf("empty serial")
	}
	parsed, err := strconv.ParseInt(sanitized, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse serial %q: %w", value, err)
	}
	return parsed, nil
}

func excelSerialToDate(serial int64) (string, error) {
	if serial <= 0 {
		return "", fmt.Errorf("invalid excel serial %d", serial)
	}

	days := serial
	if serial > 59 {
		days--
	}

	base := time.Date(1899, 12, 31, 0, 0, 0, 0, time.UTC)
	return base.AddDate(0, 0, int(days)).Format("2006-01-02"), nil
}

func parseEuroMinorUnits(value string) (int64, error) {
	cleaned := strings.TrimSpace(value)
	cleaned = strings.NewReplacer("€", "", "\u00a0", "", " ", "").Replace(cleaned)
	if cleaned == "" {
		return 0, fmt.Errorf("empty value")
	}

	sign := int64(1)
	switch cleaned[0] {
	case '-':
		sign = -1
		cleaned = cleaned[1:]
	case '+':
		cleaned = cleaned[1:]
	}

	whole, fraction, err := splitDecimal(cleaned)
	if err != nil {
		return 0, err
	}

	wholeValue, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse integer part: %w", err)
	}
	cents, err := scaleFraction(wholeValue, fraction)
	if err != nil {
		return 0, err
	}
	if sign < 0 {
		cents = -cents
	}
	return cents, nil
}

func splitDecimal(value string) (string, string, error) {
	lastDot := strings.LastIndex(value, ".")
	lastComma := strings.LastIndex(value, ",")
	sep := maxInt(lastDot, lastComma)

	if sep < 0 {
		if !digitsOnly(value) {
			return "", "", fmt.Errorf("invalid number %q", value)
		}
		return value, "", nil
	}

	whole := strings.ReplaceAll(value[:sep], ".", "")
	whole = strings.ReplaceAll(whole, ",", "")
	fraction := strings.ReplaceAll(value[sep+1:], ".", "")
	fraction = strings.ReplaceAll(fraction, ",", "")

	if whole == "" {
		whole = "0"
	}
	if !digitsOnly(whole) || (fraction != "" && !digitsOnly(fraction)) {
		return "", "", fmt.Errorf("invalid number %q", value)
	}
	return whole, fraction, nil
}

func scaleFraction(whole int64, fraction string) (int64, error) {
	if whole > (1<<63-1)/100 {
		return 0, fmt.Errorf("amount overflow")
	}
	base := whole * 100
	if fraction == "" {
		return base, nil
	}

	if len(fraction) == 1 {
		fraction += "0"
	}
	two := fraction
	if len(two) > 2 {
		two = two[:2]
	}
	cents, err := strconv.ParseInt(two, 10, 64)
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

func normalizeHeader(value string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func rowIsBlank(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

func isTotalRow(row []string) bool {
	for _, cell := range row {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(cell)), "totale") {
			return true
		}
	}
	return false
}

func field(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func widibaError(line int, message string, raw map[string]string) fuiproto.ErrorRecord {
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

func normalizeSpaces(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func digitsOnly(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxDate(a, b string) string {
	if a > b {
		return a
	}
	return b
}

func fallbackLatestDate(movements []widibaMovement) string {
	latest := "1970-01-01"
	for _, movement := range movements {
		latest = maxDate(latest, movement.date)
	}
	return latest
}

func extractFirstIBAN(value string) string {
	match := ibanRegex.FindString(strings.ToUpper(value))
	return strings.TrimSpace(match)
}

func extractPayee(description string) string {
	candidates := []*regexp.Regexp{payeeAFavoreRegex, payeeOrdRegex}
	for _, re := range candidates {
		matches := re.FindStringSubmatch(description)
		if len(matches) == 2 {
			value := trimPayee(matches[1])
			if value != "" {
				return value
			}
		}
	}
	return ""
}

func trimPayee(value string) string {
	payee := normalizeSpaces(value)
	if iban := extractFirstIBAN(payee); iban != "" {
		payee = strings.TrimSpace(strings.Replace(payee, iban, "", 1))
	}
	return normalizeSpaces(payee)
}

func widibaKind(causale string) fuiproto.TransactionKind {
	switch normalizeSpaces(strings.ToLower(causale)) {
	case "bonifico in entrata":
		return fuiproto.TransactionKindTransfer
	case "bonifico estero in uscita":
		return fuiproto.TransactionKindTransfer
	case "pagamento istantaneo":
		return fuiproto.TransactionKindTransfer
	case "addebito diretto sdd":
		return fuiproto.TransactionKindPayment
	case "trasferimento a / da linea libera":
		return fuiproto.TransactionKindTransfer
	case "rimborso titoli e/o fondi comuni":
		return fuiproto.TransactionKindInvestment
	default:
		return fuiproto.TransactionKindOther
	}
}

func isLineaLibera(causale string) bool {
	return normalizeSpaces(strings.ToLower(causale)) == "trasferimento a / da linea libera"
}

func widibaMovementID(
	date string,
	causale string,
	description string,
	amount int64,
	fallbackIDIndex map[string]int,
) string {
	if code, ok := operationCode(description); ok {
		return "widiba-op-" + strings.ToLower(code)
	}

	fingerprint := fmt.Sprintf("%s|%s|%s|%d", date, causale, description, amount)
	fallbackIDIndex[fingerprint]++
	index := fallbackIDIndex[fingerprint]
	return hashValues(fingerprint, strconv.Itoa(index))
}

func operationCode(description string) (string, bool) {
	patterns := []*regexp.Regexp{bonIstRegex, bonSepaRegex, sddRegex}
	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(description)
		if len(matches) == 2 {
			return sanitizeIDToken(matches[1]), true
		}
	}
	return "", false
}

func sanitizeIDToken(value string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(value) {
		if unicode.IsDigit(r) || (r >= 'A' && r <= 'Z') {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

func accountSuffix(iban string) string {
	digits := make([]rune, 0, len(iban))
	for _, r := range iban {
		if unicode.IsDigit(r) {
			digits = append(digits, r)
		}
	}
	if len(digits) >= 4 {
		return string(digits[len(digits)-4:])
	}

	alnum := sanitizeIDToken(iban)
	if len(alnum) >= 4 {
		return strings.ToLower(alnum[len(alnum)-4:])
	}
	return "acct"
}

func hashValues(values ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(values, "|")))
	return hex.EncodeToString(sum[:])
}

func sha256Hex(source []byte) string {
	sum := sha256.Sum256(source)
	return hex.EncodeToString(sum[:])
}
