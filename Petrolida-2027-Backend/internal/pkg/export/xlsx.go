package export

import (
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"
)

const xlsxSheetName = "Pendaftar"

func WriteXLSX(w io.Writer, table Table) error {
	file := excelize.NewFile()
	defer file.Close()
	if table.Review != nil {
		if err := writeReviewWorkbook(file, *table.Review); err != nil {
			return err
		}
		_, err := file.WriteTo(w)
		return err
	}

	sheets := table.Sheets
	if len(sheets) == 0 {
		sheets = []Sheet{{Name: xlsxSheetName, Rows: table.Rows}}
	}
	used := map[string]bool{}
	for i, sheet := range sheets {
		name := uniqueSheetName(sheet.Name, used)
		if i == 0 {
			if err := file.SetSheetName("Sheet1", name); err != nil {
				return err
			}
		} else if _, err := file.NewSheet(name); err != nil {
			return err
		}
		columns := sheet.Columns
		if len(columns) == 0 {
			columns = table.Columns
		}
		if err := writeXLSXSheet(file, name, Table{Columns: columns, Rows: sheet.Rows}); err != nil {
			return err
		}
	}
	_, err := file.WriteTo(w)
	return err
}

func writeXLSXSheet(file *excelize.File, name string, table Table) error {
	stream, err := file.NewStreamWriter(name)
	if err != nil {
		return err
	}
	headerStyle, err := file.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return err
	}
	// Pengaturan panes dan lebar kolom harus dilakukan sebelum baris pertama ditulis.
	if err := stream.SetPanes(&excelize.Panes{
		Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft",
	}); err != nil {
		return err
	}
	if len(table.Columns) > 0 {
		if err := stream.SetColWidth(1, len(table.Columns), 24); err != nil {
			return err
		}
	}

	header := make([]any, len(table.Columns))
	for i, column := range table.Columns {
		header[i] = excelize.Cell{StyleID: headerStyle, Value: xlsxCell(SafeCell(column.Header))}
	}
	if err := stream.SetRow("A1", header); err != nil {
		return err
	}

	for rowIndex, row := range table.Rows {
		values := make([]any, len(table.Columns))
		for i, column := range table.Columns {
			value := ""
			if i < len(row) {
				value = row[i]
			}
			// Nilai string ditulis sebagai sel teks, jadi NRP/telepon yang lolos
			// literalText tidak perlu trik ="..." seperti di CSV.
			if !column.Text || !literalText.MatchString(value) {
				value = SafeCell(value)
			}
			values[i] = xlsxCell(value)
		}
		cell, err := excelize.CoordinatesToCellName(1, rowIndex+2)
		if err != nil {
			return err
		}
		if err := stream.SetRow(cell, values); err != nil {
			return err
		}
	}

	return stream.Flush()
}

func uniqueSheetName(value string, used map[string]bool) string {
	value = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(":\\/?*[]", r) {
			return '-'
		}
		return r
	}, value)
	value = strings.Trim(strings.TrimSpace(value), "'")
	if value == "" {
		value = xlsxSheetName
	}
	base := []rune(value)
	for index := 1; ; index++ {
		suffix := ""
		if index > 1 {
			suffix = fmt.Sprintf(" (%d)", index)
		}
		limit := 31 - len([]rune(suffix))
		name := string(base[:min(len(base), limit)]) + suffix
		name = strings.Trim(name, "'")
		// History is reserved by Excel; names are unique without regard to case.
		if strings.EqualFold(name, "History") || used[strings.ToLower(name)] {
			continue
		}
		used[strings.ToLower(name)] = true
		return name
	}
}

// xlsxCell memotong nilai yang melebihi batas karakter per sel Excel supaya
// satu jawaban yang sangat panjang tidak menggagalkan seluruh export.
func xlsxCell(value string) string {
	runes := []rune(value)
	if len(runes) <= excelize.TotalCellChars {
		return value
	}
	const marker = "…"
	return string(runes[:excelize.TotalCellChars-len([]rune(marker))]) + marker
}
