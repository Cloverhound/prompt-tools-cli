package bulk

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Row represents a single prompt row from the spreadsheet.
type Row struct {
	LineNumber int
	Filename   string
	Voice      string
	Text       string
	IsSSML     bool
	SampleRate int
	Encoding   string
	Notes      string
}

// ParseFile reads an xlsx or csv file and returns prompt rows.
func ParseFile(filePath, sheetName string) ([]Row, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".xlsx":
		return parseExcel(filePath, sheetName)
	case ".csv":
		return parseCSV(filePath)
	default:
		return nil, fmt.Errorf("unsupported file format: %s (use .xlsx or .csv)", ext)
	}
}

func parseExcel(filePath, sheetName string) ([]Row, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening Excel file: %w", err)
	}
	defer f.Close()

	if sheetName == "" {
		sheetName = f.GetSheetName(0)
	}

	excelRows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("reading sheet %q: %w", sheetName, err)
	}

	return parseRawRows(excelRows)
}

func parseCSV(filePath string) ([]Row, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading CSV: %w", err)
	}

	return parseRawRows(records)
}

func parseRawRows(records [][]string) ([]Row, error) {
	if len(records) < 2 {
		return nil, fmt.Errorf("file must have a header row and at least one data row")
	}

	var rows []Row
	for i := 1; i < len(records); i++ { // skip header
		record := records[i]
		if len(record) == 0 {
			continue
		}

		filename := getField(record, 0)
		if filename == "" {
			continue
		}
		// Skip rows starting with #
		if strings.HasPrefix(filename, "#") {
			continue
		}

		row := Row{
			LineNumber: i + 1,
			Filename:   filename,
			Voice:      getField(record, 1),
			Text:       getField(record, 2),
			Notes:      getField(record, 6),
		}

		// SSML column
		ssmlField := strings.ToLower(getField(record, 3))
		row.IsSSML = ssmlField == "yes" || ssmlField == "true" || ssmlField == "1"

		// Sample rate override
		if sr := getField(record, 4); sr != "" {
			fmt.Sscanf(sr, "%d", &row.SampleRate)
		}

		// Encoding override
		row.Encoding = getField(record, 5)

		rows = append(rows, row)
	}

	return rows, nil
}

func getField(record []string, index int) string {
	if index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}
