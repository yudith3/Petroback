package export

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func openXLSX(t *testing.T, table Table) *excelize.File {
	t.Helper()
	var buffer bytes.Buffer
	if err := WriteXLSX(&buffer, table); err != nil {
		t.Fatalf("WriteXLSX() error = %v", err)
	}
	file, err := excelize.OpenReader(&buffer)
	if err != nil {
		t.Fatalf("open XLSX: %v", err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}

func TestWriteXLSXEmptyTableWritesHeaderOnly(t *testing.T) {
	file := openXLSX(t, Table{Columns: []Column{{Header: "Nama"}, {Header: "NRP", Text: true}}})
	rows, err := file.GetRows(xlsxSheetName)
	if err != nil {
		t.Fatalf("GetRows() error = %v", err)
	}
	if len(rows) != 1 || !slices.Equal(rows[0], []string{"Nama", "NRP"}) {
		t.Fatalf("rows = %v, want header only", rows)
	}
}

func TestWriteXLSXKeepsTextColumnsAsStrings(t *testing.T) {
	file := openXLSX(t, Table{
		Columns: []Column{{Header: "NRP", Text: true}, {Header: "No. Telepon", Text: true}},
		Rows:    [][]string{{"5025251178", "081234567890"}, {"0123456789012345", "+6281234567890"}},
	})
	want := map[string]string{
		"A2": "5025251178", "B2": "081234567890",
		"A3": "0123456789012345", "B3": "+6281234567890",
	}
	for cell, value := range want {
		got, err := file.GetCellValue(xlsxSheetName, cell)
		if err != nil || got != value {
			t.Errorf("%s = %q (err %v), want %q", cell, got, err, value)
		}
		cellType, err := file.GetCellType(xlsxSheetName, cell)
		if err != nil {
			t.Fatalf("GetCellType(%s) error = %v", cell, err)
		}
		if cellType != excelize.CellTypeInlineString && cellType != excelize.CellTypeSharedString {
			t.Errorf("%s type = %v, want string cell", cell, cellType)
		}
	}
}

func TestWriteXLSXNeutralizesFormula(t *testing.T) {
	values := []string{"=1+1", "+62 812", "-2", "@SUM(A1)", "\t=1", `=HYPERLINK("http://x","klik")`}
	rows := make([][]string, 0, len(values))
	for _, value := range values {
		rows = append(rows, []string{value})
	}
	file := openXLSX(t, Table{Columns: []Column{{Header: "Jawaban"}}, Rows: rows})
	for i, value := range values {
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		formula, err := file.GetCellFormula(xlsxSheetName, cell)
		if err != nil || formula != "" {
			t.Errorf("%s has formula %q (err %v)", cell, formula, err)
		}
		got, _ := file.GetCellValue(xlsxSheetName, cell)
		if got != "'"+value {
			t.Errorf("%s = %q, want %q", cell, got, "'"+value)
		}
	}
}

func TestWriteXLSXKeepsSpecialCharacters(t *testing.T) {
	values := []string{"Budi, S.T.", "baris 1\nbaris 2", `kata "kutip"`, "Zoë 🚀 石油"}
	file := openXLSX(t, Table{
		Columns: []Column{{Header: "A"}, {Header: "B"}, {Header: "C"}, {Header: "D"}},
		Rows:    [][]string{values},
	})
	rows, err := file.GetRows(xlsxSheetName)
	if err != nil {
		t.Fatalf("GetRows() error = %v", err)
	}
	if !slices.Equal(rows[1], values) {
		t.Fatalf("row = %q, want %q", rows[1], values)
	}
}

func TestWriteXLSXTruncatesOversizedCell(t *testing.T) {
	file := openXLSX(t, Table{
		Columns: []Column{{Header: "Jawaban"}},
		Rows:    [][]string{{strings.Repeat("a", excelize.TotalCellChars+10)}},
	})
	got, err := file.GetCellValue(xlsxSheetName, "A2")
	if err != nil {
		t.Fatalf("GetCellValue() error = %v", err)
	}
	if len([]rune(got)) != excelize.TotalCellChars || !strings.HasSuffix(got, "…") {
		t.Fatalf("cell length = %d, want %d with marker", len([]rune(got)), excelize.TotalCellChars)
	}
}

func TestWriteXLSXMultipleSheetsHaveSafeUniqueNamesAndTextCells(t *testing.T) {
	file := openXLSX(t, Table{
		Columns: []Column{{Header: "NRP", Text: true}, {Header: "Answer"}},
		Sheets: []Sheet{
			{Name: "Creative/Design", Rows: [][]string{{"000123456789012345", "=1+1"}}},
			{Name: "creative/design", Rows: [][]string{{"0123", "secondary"}}},
			{Name: strings.Repeat("石", 40)},
			{Name: "History"},
			{Name: "'"},
		},
	})
	names := file.GetSheetList()
	if len(names) != 5 || names[0] != "Creative-Design" || names[1] != "creative-design (2)" || len([]rune(names[2])) != 31 || names[3] != "History (2)" || names[4] != "Pendaftar" {
		t.Fatalf("sheet names: %v", names)
	}
	for _, name := range names {
		rows, err := file.GetRows(name)
		if err != nil || len(rows) == 0 || !slices.Equal(rows[0], []string{"NRP", "Answer"}) {
			t.Fatalf("header for %q: %v %v", name, rows, err)
		}
	}
	value, _ := file.GetCellValue(names[0], "A2")
	formula, _ := file.GetCellFormula(names[0], "B2")
	answer, _ := file.GetCellValue(names[0], "B2")
	if value != "000123456789012345" || formula != "" || answer != "'=1+1" {
		t.Fatalf("unsafe cell values: %q %q %q", value, formula, answer)
	}
}
