package bulk

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

var templateHeaders = []string{"Filename", "Voice", "Text", "SSML", "Sample Rate", "Encoding", "Notes"}

var templateRows = [][]string{
	{"welcome.wav", "en-US-Neural2-F", "Welcome to our support line.", "no", "", "", "Main greeting"},
	{"#holiday.wav", "en-US-Neural2-F", "We are closed for the holiday.", "no", "", "", "Skipped row example"},
	{"transfer.wav", "en-US-Neural2-F", "<speak>Please hold while we transfer your call.<break time=\"500ms\"/></speak>", "yes", "16000", "linear16", "SSML example"},
}

// GenerateTemplate creates a blank template file.
func GenerateTemplate(outputPath string) error {
	ext := strings.ToLower(filepath.Ext(outputPath))
	switch ext {
	case ".xlsx":
		return generateExcelTemplate(outputPath)
	case ".csv":
		return generateCSVTemplate(outputPath)
	default:
		return fmt.Errorf("unsupported format: %s (use .xlsx or .csv)", ext)
	}
}

func generateExcelTemplate(outputPath string) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Sheet1"

	// Write headers
	for i, h := range templateHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	// Write sample rows
	for rowIdx, row := range templateRows {
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			f.SetCellValue(sheet, cell, val)
		}
	}

	// Set column widths
	widths := map[string]float64{"A": 20, "B": 22, "C": 50, "D": 6, "E": 14, "F": 12, "G": 25}
	for col, w := range widths {
		f.SetColWidth(sheet, col, col, w)
	}

	return f.SaveAs(outputPath)
}

func generateCSVTemplate(outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	w := csv.NewWriter(file)
	if err := w.Write(templateHeaders); err != nil {
		return err
	}
	for _, row := range templateRows {
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
