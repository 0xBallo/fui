package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestRunCSV(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "input.csv")
	outputPath := filepath.Join(t.TempDir(), "output.csv")

	content := "Type,Description\nTransfer,Transfer to Mario Rossi IT60X0542811101000000123456\nTransfer,Transfer to Mario Rossi IT60X0542811101000000123456\n"
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write input CSV: %v", err)
	}

	if err := run(inputPath, outputPath); err != nil {
		t.Fatalf("run anonymizer: %v", err)
	}

	out, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output CSV: %v", err)
	}
	text := string(out)
	if strings.Contains(text, "Mario Rossi") || strings.Contains(text, "IT60X0542811101000000123456") {
		t.Fatalf("output still contains sensitive values: %s", text)
	}
	if strings.Count(text, "Transfer to Person 001") != 2 {
		t.Fatalf("expected deterministic replacement in output: %s", text)
	}
}

func TestRunXLSX(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "input.xlsx")
	outputPath := filepath.Join(t.TempDir(), "output.xlsx")

	file := excelize.NewFile()
	defer func() {
		_ = file.Close()
	}()
	_ = file.SetSheetName("Sheet1", "Movimenti conto")
	_ = file.SetCellValue("Movimenti conto", "A1", "IBAN")
	_ = file.SetCellValue("Movimenti conto", "B1", "IT60X0542811101000000123456")
	_ = file.SetCellValue("Movimenti conto", "A2", "Intestatario")
	_ = file.SetCellValue("Movimenti conto", "B2", "Mario Rossi")
	_ = file.SetCellValue("Movimenti conto", "A3", "DESCRIZIONE")
	_ = file.SetCellValue("Movimenti conto", "B3", "A Favore Mario Rossi")

	if err := file.SaveAs(inputPath); err != nil {
		t.Fatalf("save input XLSX: %v", err)
	}

	if err := run(inputPath, outputPath); err != nil {
		t.Fatalf("run anonymizer: %v", err)
	}

	anonFile, err := excelize.OpenFile(outputPath)
	if err != nil {
		t.Fatalf("open output XLSX: %v", err)
	}
	defer func() {
		_ = anonFile.Close()
	}()

	iban, err := anonFile.GetCellValue("Movimenti conto", "B1")
	if err != nil {
		t.Fatalf("read anonymized IBAN: %v", err)
	}
	if iban == "IT60X0542811101000000123456" {
		t.Fatalf("IBAN was not anonymized")
	}
	if header, err := anonFile.GetCellValue("Movimenti conto", "A3"); err != nil || header != "DESCRIZIONE" {
		t.Fatalf("header changed unexpectedly: %q (%v)", header, err)
	}
}
