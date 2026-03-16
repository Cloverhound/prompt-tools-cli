package bulk

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidationError represents a row validation error.
type ValidationError struct {
	Row     int
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("row %d: %s: %s", e.Row, e.Field, e.Message)
}

// ValidateRows checks all rows for errors.
func ValidateRows(rows []Row) []error {
	var errs []error
	filenames := make(map[string]int) // track duplicate filenames

	for _, row := range rows {
		// Filename is required
		if row.Filename == "" {
			errs = append(errs, &ValidationError{Row: row.LineNumber, Field: "Filename", Message: "required"})
			continue
		}

		// Check for valid extension
		ext := strings.ToLower(filepath.Ext(row.Filename))
		if ext != ".wav" && ext != ".mp3" {
			errs = append(errs, &ValidationError{Row: row.LineNumber, Field: "Filename", Message: "must end in .wav or .mp3"})
		}

		// Check for duplicate filenames
		if prev, ok := filenames[strings.ToLower(row.Filename)]; ok {
			errs = append(errs, &ValidationError{Row: row.LineNumber, Field: "Filename", Message: fmt.Sprintf("duplicate of row %d", prev)})
		}
		filenames[strings.ToLower(row.Filename)] = row.LineNumber

		// Text is required
		if row.Text == "" {
			errs = append(errs, &ValidationError{Row: row.LineNumber, Field: "Text", Message: "required"})
		}

		// Validate sample rate if specified
		if row.SampleRate != 0 {
			valid := map[int]bool{8000: true, 16000: true, 22050: true, 24000: true}
			if !valid[row.SampleRate] {
				errs = append(errs, &ValidationError{Row: row.LineNumber, Field: "Sample Rate", Message: "must be 8000, 16000, 22050, or 24000"})
			}
		}

		// Validate encoding if specified
		if row.Encoding != "" {
			valid := map[string]bool{"mulaw": true, "alaw": true, "linear16": true, "mp3": true}
			if !valid[strings.ToLower(row.Encoding)] {
				errs = append(errs, &ValidationError{Row: row.LineNumber, Field: "Encoding", Message: "must be mulaw, alaw, linear16, or mp3"})
			}
		}
	}

	return errs
}
