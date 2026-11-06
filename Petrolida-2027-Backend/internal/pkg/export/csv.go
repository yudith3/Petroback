package export

import (
	"encoding/csv"
	"io"
	"regexp"
	"strings"
	"time"
)

// Indonesia tidak memakai DST, jadi zona tetap +7 cukup dan tidak bergantung
// pada data zona waktu di image Docker.
var Jakarta = time.FixedZone("WIB", 7*60*60)

type Column struct {
	Header string
	// Text menandai kolom seperti NRP dan telepon yang harus tetap teks supaya
	// nol di depan tidak hilang dan tidak berubah jadi notasi ilmiah.
	Text bool
	// Input keeps assessment cells editable with a general number format.
	Input bool
	// Score identifies assessment inputs included in the automatic total.
	Score bool
	// SumScores writes a trusted formula that totals only Score columns.
	SumScores bool
	// Link displays an HTTP(S) document URL as a short hyperlink in Excel.
	Link bool
}

type Table struct {
	Columns []Column
	Rows    [][]string
	Sheets  []Sheet
	Review  *ReviewWorkbook
}

// CSV uses Rows; simple Excel tables use Sheets. Review workbooks add navigation
// and independent data, answers, and scoring sections for each parent division.
type Sheet struct {
	Name    string
	Columns []Column
	Rows    [][]string
}

type ReviewWorkbook struct {
	RegistrantCount int
	Divisions       []DivisionWorkbook
}

type DivisionWorkbook struct {
	Name    string
	Data    Sheet
	Answers Sheet
	Scoring Sheet
}

// utf8BOM membuat Excel membaca file sebagai UTF-8, bukan ANSI.
const utf8BOM = "\xEF\xBB\xBF"

var literalText = regexp.MustCompile(`^[A-Za-z0-9.+-]+$`)

// SafeCell menetralkan nilai yang akan dibaca spreadsheet sebagai formula.
func SafeCell(value string) string {
	if value != "" && strings.ContainsRune("=+-@\t\r", rune(value[0])) {
		return "'" + value
	}
	return value
}

// textCell membungkus nilai sebagai formula konstanta ="...". Hanya dipakai
// untuk nilai yang lolos literalText, jadi tidak ada input yang bisa tereksekusi.
func textCell(value string) string {
	if literalText.MatchString(value) {
		return `="` + value + `"`
	}
	return SafeCell(value)
}

func WriteCSV(w io.Writer, table Table) error {
	if _, err := io.WriteString(w, utf8BOM); err != nil {
		return err
	}
	writer := csv.NewWriter(w)

	header := make([]string, len(table.Columns))
	for i, column := range table.Columns {
		header[i] = SafeCell(column.Header)
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	record := make([]string, len(table.Columns))
	for _, row := range table.Rows {
		for i, column := range table.Columns {
			value := ""
			if i < len(row) {
				value = row[i]
			}
			if column.Text {
				record[i] = textCell(value)
			} else {
				record[i] = SafeCell(value)
			}
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	writer.Flush()
	return writer.Error()
}
