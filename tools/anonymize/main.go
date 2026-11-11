// Package main provides a CLI to anonymize Revolut CSV and Widiba XLSX exports.
package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/xuri/excelize/v2"
)

var (
	ibanPattern          = regexp.MustCompile(`(?i)\b[A-Z]{2}\d{2}[A-Z0-9]{10,30}\b`)
	revolutToPattern     = regexp.MustCompile(`(?i)^(transfer to\s+)(.+)$`)
	revolutFromPattern   = regexp.MustCompile(`(?i)^(transfer from\s+)(.+)$`)
	widibaAFavorePattern = regexp.MustCompile(`(?i)(A\s*Favore\s*[:\-]?\s*)([^;\n]+)`)
	widibaOrdPattern     = regexp.MustCompile(`(?i)(Ord:\s*)([^;\n]+)`)
)

type anonymizer struct {
	nameMap map[string]string
	descMap map[string]string
	ibanMap map[string]string
	nameSeq int
	descSeq int
	ibanSeq int
}

func newAnonymizer() *anonymizer {
	return &anonymizer{
		nameMap: map[string]string{},
		descMap: map[string]string{},
		ibanMap: map[string]string{},
	}
}

func main() {
	var inputPath string
	var outputPath string

	flag.StringVar(&inputPath, "input", "", "input file path (.csv or .xlsx)")
	flag.StringVar(&outputPath, "output", "", "output anonymized file path")
	flag.Parse()

	if inputPath == "" || outputPath == "" {
		fmt.Fprintln(os.Stderr, "usage: anonymize --input <file> --output <file>")
		os.Exit(2)
	}

	if err := run(inputPath, outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "anonymize: %v\n", err)
		os.Exit(1)
	}
}

func run(inputPath, outputPath string) error {
	// #nosec G304 -- inputPath is an explicit local CLI argument selected by the user.
	content, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read input file: %w", err)
	}

	anon := newAnonymizer()
	ext := strings.ToLower(filepath.Ext(inputPath))
	switch ext {
	case ".csv":
		out, err := anonymizeCSV(content, anon)
		if err != nil {
			return err
		}
		if err := os.WriteFile(outputPath, out, 0o600); err != nil {
			return fmt.Errorf("write output CSV: %w", err)
		}
		return nil
	case ".xlsx":
		out, err := anonymizeXLSX(content, anon)
		if err != nil {
			return err
		}
		if err := os.WriteFile(outputPath, out, 0o600); err != nil {
			return fmt.Errorf("write output XLSX: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported file extension %q", ext)
	}
}

func anonymizeCSV(content []byte, anon *anonymizer) ([]byte, error) {
	reader := csv.NewReader(bytes.NewReader(content))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("decode CSV: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("empty CSV")
	}

	headers := rows[0]
	descriptionColumn := -1
	for i, header := range headers {
		if strings.EqualFold(strings.TrimSpace(header), "description") {
			descriptionColumn = i
			break
		}
	}

	for i := 1; i < len(rows); i++ {
		for j := range rows[i] {
			value := rows[i][j]
			if descriptionColumn >= 0 && j == descriptionColumn {
				rows[i][j] = anon.anonymizeDescriptionCell(value)
				continue
			}
			rows[i][j] = anon.replaceIBANs(value)
		}
	}

	var out bytes.Buffer
	writer := csv.NewWriter(&out)
	if err := writer.WriteAll(rows); err != nil {
		return nil, fmt.Errorf("encode CSV: %w", err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("flush CSV writer: %w", err)
	}
	return out.Bytes(), nil
}

func anonymizeXLSX(content []byte, anon *anonymizer) ([]byte, error) {
	workbook, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer func() {
		_ = workbook.Close()
	}()

	for _, sheet := range workbook.GetSheetList() {
		rows, err := workbook.GetRows(sheet)
		if err != nil {
			return nil, fmt.Errorf("read sheet %q rows: %w", sheet, err)
		}
		for rowIndex, row := range rows {
			for colIndex, value := range row {
				replaced := anon.anonymizeWorkbookCell(value)
				if replaced == value {
					continue
				}
				cell, err := excelize.CoordinatesToCellName(colIndex+1, rowIndex+1)
				if err != nil {
					return nil, fmt.Errorf("cell coordinates: %w", err)
				}
				if err := workbook.SetCellValue(sheet, cell, replaced); err != nil {
					return nil, fmt.Errorf("set cell %s: %w", cell, err)
				}
			}
		}
	}

	var out bytes.Buffer
	if err := workbook.Write(&out); err != nil {
		return nil, fmt.Errorf("write xlsx: %w", err)
	}
	return out.Bytes(), nil
}

func (a *anonymizer) anonymizeWorkbookCell(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return value
	}
	if isHeaderOrLabel(trimmed) || isDateOrAmount(trimmed) {
		return a.replaceIBANs(value)
	}

	replaced := a.replaceIBANs(value)
	replaced = a.replaceTaggedNames(replaced)
	if replaced != value {
		return replaced
	}
	if looksLikeName(trimmed) {
		return a.nameAlias(trimmed)
	}
	if looksLikeDescription(trimmed) {
		return a.descriptionAlias(trimmed)
	}
	return value
}

func (a *anonymizer) anonymizeDescriptionCell(value string) string {
	replaced := a.replaceIBANs(value)
	replaced = a.replaceTaggedNames(replaced)
	if replaced != value {
		return replaced
	}
	return a.descriptionAlias(normalizeSpaces(value))
}

func (a *anonymizer) replaceTaggedNames(value string) string {
	value = replacePattern(value, revolutToPattern, func(found string) string {
		matches := revolutToPattern.FindStringSubmatch(found)
		if len(matches) != 3 {
			return found
		}
		return matches[1] + a.nameAlias(normalizeSpaces(matches[2]))
	})
	value = replacePattern(value, revolutFromPattern, func(found string) string {
		matches := revolutFromPattern.FindStringSubmatch(found)
		if len(matches) != 3 {
			return found
		}
		return matches[1] + a.nameAlias(normalizeSpaces(matches[2]))
	})
	value = replacePattern(value, widibaAFavorePattern, func(found string) string {
		matches := widibaAFavorePattern.FindStringSubmatch(found)
		if len(matches) != 3 {
			return found
		}
		return matches[1] + a.nameAlias(normalizeSpaces(matches[2]))
	})
	value = replacePattern(value, widibaOrdPattern, func(found string) string {
		matches := widibaOrdPattern.FindStringSubmatch(found)
		if len(matches) != 3 {
			return found
		}
		return matches[1] + a.nameAlias(normalizeSpaces(matches[2]))
	})
	return value
}

func (a *anonymizer) replaceIBANs(value string) string {
	return replacePattern(value, ibanPattern, func(found string) string {
		return a.ibanAlias(found)
	})
}

func (a *anonymizer) nameAlias(name string) string {
	key := strings.ToUpper(strings.TrimSpace(name))
	if alias, ok := a.nameMap[key]; ok {
		return alias
	}
	a.nameSeq++
	alias := fmt.Sprintf("Person %03d", a.nameSeq)
	a.nameMap[key] = alias
	return alias
}

func (a *anonymizer) descriptionAlias(description string) string {
	key := strings.ToUpper(strings.TrimSpace(description))
	if alias, ok := a.descMap[key]; ok {
		return alias
	}
	a.descSeq++
	alias := fmt.Sprintf("Description %03d", a.descSeq)
	a.descMap[key] = alias
	return alias
}

func (a *anonymizer) ibanAlias(iban string) string {
	key := strings.ToUpper(strings.TrimSpace(iban))
	if alias, ok := a.ibanMap[key]; ok {
		return alias
	}

	country := "IT"
	if len(key) >= 2 && unicode.IsLetter(rune(key[0])) && unicode.IsLetter(rune(key[1])) {
		country = key[:2]
	}
	targetLen := len(key)
	if targetLen < 16 {
		targetLen = 16
	}
	a.ibanSeq++
	alias := country + "00" + alnumPad(a.ibanSeq, targetLen-4)
	a.ibanMap[key] = alias
	return alias
}

func alnumPad(index, length int) string {
	value := strings.ToUpper(strconv.FormatInt(int64(index), 36))
	if len(value) >= length {
		return value[len(value)-length:]
	}
	return strings.Repeat("X", length-len(value)) + value
}

func replacePattern(value string, re *regexp.Regexp, replace func(found string) string) string {
	if !re.MatchString(value) {
		return value
	}

	var out strings.Builder
	last := 0
	for _, match := range re.FindAllStringIndex(value, -1) {
		out.WriteString(value[last:match[0]])
		out.WriteString(replace(value[match[0]:match[1]]))
		last = match[1]
	}
	out.WriteString(value[last:])
	return out.String()
}

func isHeaderOrLabel(value string) bool {
	headers := map[string]struct{}{
		"DATA CONT.":      {},
		"DATA VAL.":       {},
		"CAUSALE":         {},
		"DESCRIZIONE":     {},
		"IMPORTO (€)":     {},
		"TOTALE":          {},
		"IBAN":            {},
		"SALDO INIZIALE":  {},
		"SALDO FINALE":    {},
		"INTESTATARIO":    {},
		"MOVIMENTI CONTO": {},
	}
	_, ok := headers[strings.ToUpper(normalizeSpaces(value))]
	return ok
}

func isDateOrAmount(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if _, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64); err == nil {
		return true
	}
	layouts := []string{"2006-01-02", "02/01/2006", "2/1/2006"}
	for _, layout := range layouts {
		if _, err := time.Parse(layout, value); err == nil {
			return true
		}
	}
	return false
}

func looksLikeName(value string) bool {
	if strings.ContainsAny(value, "0123456789") {
		return false
	}
	words := strings.Fields(value)
	if len(words) < 2 || len(words) > 4 {
		return false
	}
	letters := 0
	for _, r := range value {
		if unicode.IsLetter(r) {
			letters++
		}
	}
	return letters >= 4
}

func looksLikeDescription(value string) bool {
	if len(value) < 6 {
		return false
	}
	return strings.ContainsAny(value, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
}

func normalizeSpaces(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}
