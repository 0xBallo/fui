package main

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

type fixtureOptions struct {
	summaryPaddingRows int
	closingBalance     string
}

func createWidibaFixture(t *testing.T, options fixtureOptions) string {
	t.Helper()

	file := excelize.NewFile()
	defer func() {
		_ = file.Close()
	}()

	_ = file.SetSheetName("Sheet1", widibaSheetName)

	row := 1
	setSummaryRow(file, row, "Intestatario", "Mario Rossi")
	row++
	for i := 0; i < options.summaryPaddingRows; i++ {
		row++
	}
	setSummaryRow(file, row, "IBAN", "IT60X0542811101000000123456")
	row++
	setSummaryRow(file, row, "Saldo iniziale", "1.000,00")
	row++
	closingBalance := options.closingBalance
	if closingBalance == "" {
		closingBalance = "1.829,40"
	}
	setSummaryRow(file, row, "Saldo finale", closingBalance)
	row += 2

	headerRow := row
	_ = file.SetCellValue(widibaSheetName, cell(1, headerRow), "DATA CONT.")
	_ = file.SetCellValue(widibaSheetName, cell(2, headerRow), "DATA VAL.")
	_ = file.SetCellValue(widibaSheetName, cell(3, headerRow), "CAUSALE")
	_ = file.SetCellValue(widibaSheetName, cell(4, headerRow), "DESCRIZIONE")
	_ = file.SetCellValue(widibaSheetName, cell(5, headerRow), "IMPORTO (€)")

	rows := []struct {
		date        int
		valueDate   int
		causale     string
		description string
		amount      string
	}{
		{
			date:        45757,
			valueDate:   45757,
			causale:     "Bonifico In Entrata",
			description: "Bon. Sepa. ABC123 A Favore ACME IT11A1234567890123456789012",
			amount:      "500,00",
		},
		{
			date:        45758,
			valueDate:   45758,
			causale:     "Bonifico Estero In Uscita",
			description: "Bon. Ist. XYZ987 Ord: John Doe IT22B9876543210987654321098",
			amount:      "-100,00",
		},
		{
			date:        45759,
			valueDate:   45759,
			causale:     "Addebito Diretto Sdd",
			description: "Addebito Sdd N. MANDATO77 Utility Bill",
			amount:      "-50,60",
		},
		{
			date:        45760,
			valueDate:   45760,
			causale:     "Trasferimento A / Da Linea Libera",
			description: "Trasferimento automatico",
			amount:      "-20,00",
		},
		{
			date:        45761,
			valueDate:   45761,
			causale:     "Rimborso Titoli E/o Fondi Comuni",
			description: "Rimborso fondo Alpha",
			amount:      "100,00",
		},
		{
			date:        45762,
			valueDate:   45762,
			causale:     "Altra Causale",
			description: "Some other movement",
			amount:      "200,00",
		},
		{
			date:        45763,
			valueDate:   45763,
			causale:     "Altra Causale",
			description: "Duplicated movement",
			amount:      "100,00",
		},
		{
			date:        45763,
			valueDate:   45763,
			causale:     "Altra Causale",
			description: "Duplicated movement",
			amount:      "100,00",
		},
	}

	for i, movement := range rows {
		line := headerRow + i + 1
		_ = file.SetCellValue(widibaSheetName, cell(1, line), movement.date)
		_ = file.SetCellValue(widibaSheetName, cell(2, line), movement.valueDate)
		_ = file.SetCellValue(widibaSheetName, cell(3, line), movement.causale)
		_ = file.SetCellValue(widibaSheetName, cell(4, line), movement.description)
		_ = file.SetCellValue(widibaSheetName, cell(5, line), movement.amount)
	}

	totalRow := headerRow + len(rows) + 1
	_ = file.SetCellValue(widibaSheetName, cell(1, totalRow), "Totale")

	path := filepath.Join(t.TempDir(), "widiba-sample.xlsx")
	if err := file.SaveAs(path); err != nil {
		t.Fatalf("save fixture workbook: %v", err)
	}
	return path
}

func setSummaryRow(file *excelize.File, row int, left, right string) {
	_ = file.SetCellValue(widibaSheetName, cell(1, row), left)
	_ = file.SetCellValue(widibaSheetName, cell(2, row), right)
}

func cell(column, row int) string {
	return fmt.Sprintf("%s%d", excelColumn(column), row)
}

func excelColumn(column int) string {
	value := column
	result := ""
	for value > 0 {
		value--
		result = string(rune('A'+(value%26))) + result
		value /= 26
	}
	return result
}
