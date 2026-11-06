package export

import (
	"github.com/xuri/excelize/v2"
	"slices"
	"strings"
	"testing"
)

func reviewFixture(name string) DivisionWorkbook {
	folder := "https://drive.google.com/drive/folders/" + strings.Repeat("a", 500)
	portfolio := "https://petrolida.com/api/v1/oprec-staff/fixture/documents/portfolio"
	return DivisionWorkbook{
		Name:    name,
		Data:    Sheet{Columns: []Column{{Header: "No."}, {Header: "Nama"}, {Header: "NRP", Text: true}, {Header: "Primary Divisi"}, {Header: "Link Berkas Wajib", Link: true}}, Rows: [][]string{{"1", "=1+1", "000123456789012345", name, folder}, {"2", "Zoë", "0002", name, ""}}},
		Answers: Sheet{Columns: []Column{{Header: "No."}, {Header: "Nama"}, {Header: "NRP", Text: true}, {Header: "Primary: Alasan Bergabung"}}, Rows: [][]string{{"1", "=1+1", "000123456789012345", "Baris 1\n" + strings.Repeat("jawaban panjang ", 40)}, {"2", "Zoë", "0002", "Jawaban kedua"}}},
		Scoring: Sheet{Columns: []Column{{Header: "No."}, {Header: "Nama"}, {Header: "NRP", Text: true}, {Header: "Primary Divisi"}, {Header: "Link Berkas Wajib", Link: true}, {Header: "Link Portfolio", Link: true}, {Header: "Skor Primary: Alasan Bergabung", Input: true}, {Header: "Skor Portfolio", Input: true}, {Header: "Skor KTM", Input: true}, {Header: "Skor CV", Input: true}}, Rows: [][]string{{"1", "=1+1", "000123456789012345", name, folder, portfolio, "", "", "", ""}, {"2", "Zoë", "0002", name, "", "", "", "", "", ""}}},
	}
}

func assertCellBorders(t *testing.T, style *excelize.Style) {
	t.Helper()
	for _, side := range []string{"left", "right", "top", "bottom"} {
		if !slices.ContainsFunc(style.Border, func(border excelize.Border) bool {
			return border.Type == side && border.Style == 1 && border.Color == "D6DAE1"
		}) {
			t.Errorf("missing thin %s cell border: %+v", side, style.Border)
		}
	}
}

func TestReviewWorkbookUsesCompactNeutralTables(t *testing.T) {
	division := "Creative's / Design"
	book := reviewFixture(division)
	file := openXLSX(t, Table{Review: &ReviewWorkbook{RegistrantCount: 2, Divisions: []DivisionWorkbook{book, reviewFixture(strings.ToLower(division)), reviewFixture(strings.Repeat("石", 40))}}})
	names := file.GetSheetList()
	if len(names) != 10 || names[0] != navigationSheet {
		t.Fatalf("unexpected workbook tabs: %v", names)
	}
	if raw, ok := file.Pkg.Load("xl/worksheets/sheet2.xml"); !ok || !strings.Contains(string(raw.([]byte)), `<autoFilter ref="$A$1:$E$3"`) {
		t.Error("filter does not start at row 1")
	}
	seen := map[string]bool{}
	for _, name := range names {
		if len([]rune(name)) > 31 || strings.ContainsAny(name, ":\\/?*[]") || seen[strings.ToLower(name)] {
			t.Fatalf("unsafe or duplicate sheet name: %q", name)
		}
		seen[strings.ToLower(name)] = true
	}
	for i, name := range names[1:] {
		cell, _ := excelize.CoordinatesToCellName(4+i%3, 2+i/3)
		exists, target, err := file.GetCellHyperLink(navigationSheet, cell)
		want := "'" + strings.ReplaceAll(name, "'", "''") + "'!A1"
		if err != nil || !exists || target != want {
			t.Errorf("navigation %s: %q, %v; want %q", cell, target, err, want)
		}
		rows, err := file.GetRows(name)
		if err != nil || len(rows) != 3 || rows[0][0] != "No." {
			t.Fatalf("%s should contain a header and one row per applicant: %v %v", name, rows, err)
		}
		merges, err := file.GetMergeCells(name)
		if err != nil || len(merges) != 0 {
			t.Errorf("%s still contains merged banners: %v", name, merges)
		}
		panes, err := file.GetPanes(name)
		if err != nil || !panes.Freeze || panes.YSplit != 1 || panes.XSplit != 3 || panes.TopLeftCell != "D2" {
			t.Errorf("freeze %s: %+v %v", name, panes, err)
		}
		props, err := file.GetSheetProps(name)
		if err != nil || props.TabColorRGB != nil {
			t.Errorf("%s still has a colored tab: %+v %v", name, props, err)
		}
		id, err := file.GetCellStyle(name, "A1")
		if err != nil {
			t.Fatal(err)
		}
		style, err := file.GetStyle(id)
		if err != nil {
			t.Fatal(err)
		}
		if style.Font == nil || !style.Font.Bold || style.Font.Color != "1F2937" || !slices.Contains(style.Fill.Color, "F3F4F6") {
			t.Errorf("overwhelming header style: %+v", style)
		}
		assertCellBorders(t, style)
		id, _ = file.GetCellStyle(name, "B2")
		style, err = file.GetStyle(id)
		if err != nil {
			t.Fatal(err)
		}
		assertCellBorders(t, style)
		if len(style.Fill.Color) != 0 {
			t.Errorf("body still has colored fills: %+v", style.Fill)
		}
		formula, _ := file.GetCellFormula(name, "B2")
		value, _ := file.GetCellValue(name, "B2")
		if formula != "" || value != "'=1+1" {
			t.Errorf("unsafe answer in %s: %q %q", name, formula, value)
		}
		value, _ = file.GetCellValue(name, "C2")
		if value != "000123456789012345" {
			t.Errorf("NRP changed in %s: %q", name, value)
		}
	}
	for _, cell := range []string{"G2", "H2", "I2", "J2"} {
		value, _ := file.GetCellValue(names[3], cell)
		formula, _ := file.GetCellFormula(names[3], cell)
		id, _ := file.GetCellStyle(names[3], cell)
		style, err := file.GetStyle(id)
		if value != "" || formula != "" || err != nil || style.NumFmt != 0 || len(style.Fill.Color) != 0 {
			t.Errorf("scoring cell %s is not a blank neutral input: %q %+v %v", cell, value, style, err)
		}
		assertCellBorders(t, style)
	}
	for _, name := range []string{names[1], names[3]} {
		height, _ := file.GetRowHeight(name, 2)
		if height != 30 {
			t.Errorf("%s has bloated row height %v", name, height)
		}
		exists, target, _ := file.GetCellHyperLink(name, "E2")
		value, _ := file.GetCellValue(name, "E2")
		if !exists || target != book.Data.Rows[0][4] || value != "Lihat Berkas" {
			t.Errorf("document link %s: %q %q", name, value, target)
		}
	}
	value, _ := file.GetCellValue(names[2], "D2")
	if value != book.Answers.Rows[0][3] {
		t.Error("compact rows truncated an answer")
	}
	height, _ := file.GetRowHeight(names[2], 2)
	if height != 84 {
		t.Errorf("answer row height is uncontrolled: %v", height)
	}
	if file.GetActiveSheetIndex() != 0 {
		t.Error("workbook does not open on navigation")
	}
}

func TestReviewDocumentHyperlinksRejectExecutableInputs(t *testing.T) {
	book := reviewFixture("Creative")
	book.Data.Rows = [][]string{{"1", "Budi", "0001", "Creative", `=HYPERLINK("https://example.com","click")`}, {"2", "Siti", "0002", "Creative", "javascript:alert(1)"}}
	file := openXLSX(t, Table{Review: &ReviewWorkbook{Divisions: []DivisionWorkbook{book}}})
	for _, cell := range []string{"E2", "E3"} {
		exists, _, err := file.GetCellHyperLink("Data - Creative", cell)
		formula, _ := file.GetCellFormula("Data - Creative", cell)
		if err != nil || exists || formula != "" {
			t.Errorf("unsafe document input became executable in %s", cell)
		}
	}
	value, _ := file.GetCellValue("Data - Creative", "E2")
	if !strings.HasPrefix(value, "'=HYPERLINK") {
		t.Errorf("formula input was not escaped: %q", value)
	}
}

func TestReviewWorkbookEmptyResultsStillHaveNavigation(t *testing.T) {
	file := openXLSX(t, Table{Review: &ReviewWorkbook{}})
	if !slices.Equal(file.GetSheetList(), []string{"Navigasi"}) {
		t.Fatal(file.GetSheetList())
	}
	value, _ := file.GetCellValue("Navigasi", "B2")
	if value != "Tidak ada peserta" {
		t.Fatalf("empty export lacks explanation: %q", value)
	}
	book := reviewFixture("Creative")
	book.Data.Rows, book.Answers.Rows, book.Scoring.Rows = nil, nil, nil
	file = openXLSX(t, Table{Review: &ReviewWorkbook{Divisions: []DivisionWorkbook{book}}})
	for _, name := range file.GetSheetList()[1:] {
		rows, err := file.GetRows(name)
		if err != nil || len(rows) != 1 || rows[0][0] != "No." {
			t.Fatalf("empty template %s: %v %v", name, rows, err)
		}
	}
}

func TestReviewScoreTotalRecalculatesOnlyAssessmentCells(t *testing.T) {
	book := reviewFixture("Creative")
	book.Scoring = Sheet{
		Columns: []Column{
			{Header: "No."}, {Header: "Nama"}, {Header: "NRP", Text: true},
			{Header: "Skor Skills Level", Input: true, Score: true},
			{Header: "Penilai", Input: true},
			{Header: "Skor Additional Information", Input: true, Score: true},
			{Header: "Skor Perusahaan yang Terhubung", Input: true, Score: true},
			{Header: "Akumulasi Skor", SumScores: true},
			{Header: "Catatan", Input: true},
		},
		Rows: [][]string{
			{"1", "Budi", "000123456789012345", "", "", "", "", "=1+1", ""},
			{"2", "Siti", "0002", "", "", "", "", "", ""},
		},
	}
	file := openXLSX(t, Table{Review: &ReviewWorkbook{Divisions: []DivisionWorkbook{book}}})
	sheet := "Scoring - Creative"
	props, err := file.GetCalcProps()
	if err != nil || props.CalcMode == nil || *props.CalcMode != "auto" || props.FullCalcOnLoad == nil || !*props.FullCalcOnLoad {
		t.Fatalf("totals must recalculate in Excel: %+v %v", props, err)
	}
	for cell, want := range map[string]string{
		"H2": `IF(COUNT(D2,F2,G2)=0,"",SUM(D2,F2,G2))`,
		"H3": `IF(COUNT(D3,F3,G3)=0,"",SUM(D3,F3,G3))`,
	} {
		formula, err := file.GetCellFormula(sheet, cell)
		if err != nil || formula != want {
			t.Fatalf("%s formula = %q %v; want %q", cell, formula, err, want)
		}
		value, err := file.CalcCellValue(sheet, cell)
		if err != nil || value != "" {
			t.Fatalf("unscored %s total = %q %v; want blank", cell, value, err)
		}
		id, _ := file.GetCellStyle(sheet, cell)
		style, err := file.GetStyle(id)
		if err != nil || style.NumFmt != 0 {
			t.Fatalf("%s total must use numeric format: %+v %v", cell, style, err)
		}
	}
	if err := file.SetCellInt(sheet, "D2", 0); err != nil {
		t.Fatal(err)
	}
	value, err := file.CalcCellValue(sheet, "H2")
	if err != nil || value != "0" {
		t.Fatalf("a scored zero must display 0: %q %v", value, err)
	}
	for cell, value := range map[string]any{"D2": 7.5, "F2": 8, "G2": 9, "E2": 999, "I2": 999, "D3": 4} {
		if err := file.SetCellValue(sheet, cell, value); err != nil {
			t.Fatal(err)
		}
	}
	for cell, want := range map[string]string{"H2": "24.5", "H3": "4"} {
		value, err := file.CalcCellValue(sheet, cell)
		if err != nil || value != want {
			t.Errorf("%s total = %q %v; want %q", cell, value, err, want)
		}
	}
	if err := file.SetCellFloat(sheet, "F2", 2.25, -1, 64); err != nil {
		t.Fatal(err)
	}
	value, err = file.CalcCellValue(sheet, "H2")
	if err != nil || value != "18.75" {
		t.Errorf("total did not follow the edited score: %q %v", value, err)
	}
}
