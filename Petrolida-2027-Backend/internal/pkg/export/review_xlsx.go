package export

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/xuri/excelize/v2"
)

const navigationSheet = "Navigasi"

type reviewTab struct {
	name, kind string
	sheet      Sheet
}
type reviewStyles struct{ header, body, input, link int }

func writeReviewWorkbook(file *excelize.File, workbook ReviewWorkbook) error {
	mode, recalculate := "auto", true
	if err := file.SetCalcProps(&excelize.CalcPropsOptions{CalcMode: &mode, FullCalcOnLoad: &recalculate}); err != nil {
		return err
	}
	if err := file.SetSheetName("Sheet1", navigationSheet); err != nil {
		return err
	}
	styles, err := newReviewStyles(file)
	if err != nil {
		return err
	}
	used := map[string]bool{strings.ToLower(navigationSheet): true}
	tabs := make([]reviewTab, 0, len(workbook.Divisions)*3)
	for _, division := range workbook.Divisions {
		for _, section := range []reviewTab{{kind: "Data", sheet: division.Data}, {kind: "Jawaban", sheet: division.Answers}, {kind: "Scoring", sheet: division.Scoring}} {
			section.name = uniqueSheetName(section.kind+" - "+division.Name, used)
			if _, err := file.NewSheet(section.name); err != nil {
				return err
			}
			tabs = append(tabs, section)
		}
	}
	if err := writeNavigation(file, workbook, tabs, styles); err != nil {
		return err
	}
	for _, tab := range tabs {
		if err := writeReviewTab(file, tab, styles); err != nil {
			return err
		}
	}
	file.SetActiveSheet(0)
	return nil
}

func newReviewStyles(file *excelize.File) (reviewStyles, error) {
	borders := []excelize.Border{
		{Type: "left", Color: "D6DAE1", Style: 1}, {Type: "right", Color: "D6DAE1", Style: 1},
		{Type: "top", Color: "D6DAE1", Style: 1}, {Type: "bottom", Color: "D6DAE1", Style: 1},
	}
	styles := []*excelize.Style{
		{Font: &excelize.Font{Family: "Calibri", Size: 11, Bold: true, Color: "1F2937"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"F3F4F6"}, Pattern: 1}, Border: borders, Alignment: &excelize.Alignment{Vertical: "center", WrapText: true}},
		{Font: &excelize.Font{Family: "Calibri", Size: 10, Color: "1F2937"}, Border: borders, Alignment: &excelize.Alignment{Vertical: "top", WrapText: true}, NumFmt: 49},
		{Font: &excelize.Font{Family: "Calibri", Size: 10, Color: "1F2937"}, Border: borders, Alignment: &excelize.Alignment{Vertical: "top", WrapText: true}},
		{Font: &excelize.Font{Family: "Calibri", Size: 10, Color: "425B76", Underline: "single"}, Border: borders, Alignment: &excelize.Alignment{Vertical: "top"}},
	}
	ids := make([]int, len(styles))
	for i, style := range styles {
		id, err := file.NewStyle(style)
		if err != nil {
			return reviewStyles{}, err
		}
		ids[i] = id
	}
	return reviewStyles{header: ids[0], body: ids[1], input: ids[2], link: ids[3]}, nil
}

func internalLink(file *excelize.File, sheet, cell, target string) error {
	return file.SetCellHyperLink(sheet, cell, "'"+strings.ReplaceAll(target, "'", "''")+"'!A1", "Location")
}

func writeNavigation(file *excelize.File, workbook ReviewWorkbook, tabs []reviewTab, styles reviewStyles) error {
	for i, tab := range tabs {
		cell, _ := excelize.CoordinatesToCellName(4+i%3, 2+i/3)
		if err := internalLink(file, navigationSheet, cell, tab.name); err != nil {
			return err
		}
	}
	stream, err := file.NewStreamWriter(navigationSheet)
	if err != nil {
		return err
	}
	if err := stream.SetPanes(&excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return err
	}
	for i, width := range []float64{6, 36, 18, 18, 18, 18} {
		if err := stream.SetColWidth(i+1, i+1, width); err != nil {
			return err
		}
	}
	header := make([]any, 6)
	for i, title := range []string{"No.", "Divisi", "Jumlah Peserta", "Data", "Jawaban", "Scoring"} {
		header[i] = excelize.Cell{StyleID: styles.header, Value: title}
	}
	if err := stream.SetRow("A1", header, excelize.RowOpts{Height: 32}); err != nil {
		return err
	}
	for i, division := range workbook.Divisions {
		values := []any{excelize.Cell{StyleID: styles.body, Value: i + 1}, excelize.Cell{StyleID: styles.body, Value: xlsxCell(SafeCell(division.Name))}, excelize.Cell{StyleID: styles.body, Value: len(division.Data.Rows)}}
		for _, label := range []string{"Data", "Jawaban", "Scoring"} {
			values = append(values, excelize.Cell{StyleID: styles.link, Value: label})
		}
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		if err := stream.SetRow(cell, values, excelize.RowOpts{Height: 28}); err != nil {
			return err
		}
	}
	if len(workbook.Divisions) == 0 {
		if err := stream.SetRow("B2", []any{excelize.Cell{StyleID: styles.body, Value: "Tidak ada peserta"}}, excelize.RowOpts{Height: 28}); err != nil {
			return err
		}
	}
	return stream.Flush()
}

func writeReviewTab(file *excelize.File, tab reviewTab, styles reviewStyles) error {
	columns := len(tab.sheet.Columns)
	if columns == 0 {
		return fmt.Errorf("review sheet %q has no columns", tab.name)
	}
	last, _ := excelize.CoordinatesToCellName(columns, 1+len(tab.sheet.Rows))
	if err := file.AutoFilter(tab.name, "A1:"+last, nil); err != nil {
		return err
	}
	// Short document labels keep long URLs from stretching every applicant row.
	for rowIndex, row := range tab.sheet.Rows {
		for i, column := range tab.sheet.Columns {
			if !column.Link || i >= len(row) || !documentLink(row[i]) {
				continue
			}
			cell, _ := excelize.CoordinatesToCellName(i+1, rowIndex+2)
			if err := file.SetCellHyperLink(tab.name, cell, row[i], "External"); err != nil {
				return err
			}
		}
	}
	stream, err := file.NewStreamWriter(tab.name)
	if err != nil {
		return err
	}
	xSplit := min(3, columns-1)
	topLeft, _ := excelize.CoordinatesToCellName(xSplit+1, 2)
	if err := stream.SetPanes(&excelize.Panes{Freeze: true, XSplit: xSplit, YSplit: 1, TopLeftCell: topLeft, ActivePane: "bottomRight"}); err != nil {
		return err
	}
	for i, column := range tab.sheet.Columns {
		if err := stream.SetColWidth(i+1, i+1, reviewColumnWidth(column)); err != nil {
			return err
		}
	}
	header := make([]any, columns)
	for i, column := range tab.sheet.Columns {
		header[i] = excelize.Cell{StyleID: styles.header, Value: SafeCell(column.Header)}
	}
	if err := stream.SetRow("A1", header, excelize.RowOpts{Height: 44}); err != nil {
		return err
	}
	for rowIndex, row := range tab.sheet.Rows {
		values := make([]any, columns)
		for i, column := range tab.sheet.Columns {
			if column.SumScores {
				values[i] = excelize.Cell{StyleID: styles.input, Formula: reviewScoreFormula(tab.sheet.Columns, rowIndex+2), Value: ""}
				continue
			}
			value, style := "", styles.body
			if i < len(row) {
				value = row[i]
			}
			if column.Input {
				style = styles.input
			}
			if column.Link && documentLink(value) {
				style, value = styles.link, "Lihat Berkas"
				if strings.Contains(column.Header, "Portfolio") {
					value = "Lihat Portfolio"
				}
			} else if !column.Text || !literalText.MatchString(value) {
				value = SafeCell(value)
			}
			values[i] = excelize.Cell{StyleID: style, Value: xlsxCell(value)}
		}
		height := 30.0
		if tab.kind == "Jawaban" {
			height = 84
		}
		cell, _ := excelize.CoordinatesToCellName(1, rowIndex+2)
		if err := stream.SetRow(cell, values, excelize.RowOpts{Height: height}); err != nil {
			return err
		}
	}
	return stream.Flush()
}

func reviewScoreFormula(columns []Column, row int) string {
	var cells []string
	for i, column := range columns {
		if column.Score && !column.SumScores {
			cell, _ := excelize.CoordinatesToCellName(i+1, row)
			cells = append(cells, cell)
		}
	}
	if len(cells) == 0 {
		return `""`
	}
	refs := strings.Join(cells, ",")
	return fmt.Sprintf(`IF(COUNT(%s)=0,"",SUM(%s))`, refs, refs)
}

func documentLink(value string) bool {
	link, err := url.Parse(value)
	return err == nil && link.Host != "" && link.User == nil && (strings.EqualFold(link.Scheme, "https") || strings.EqualFold(link.Scheme, "http"))
}

func reviewColumnWidth(column Column) float64 {
	header := column.Header
	switch {
	case column.Link:
		return 18
	case strings.HasPrefix(header, "Skor "):
		return 28
	case header == "No.":
		return 6
	case header == "NRP":
		return 22
	case header == "Angkatan", header == "No. Telepon", header == "ID LINE":
		return 18
	case header == "Nama", header == "Penilai", strings.Contains(header, "Subdivisi"):
		return 28
	case header == "Email", header == "Departemen":
		return 32
	case header == "Catatan", header == "Tentang Petrolida", strings.Contains(header, "Pemahaman"), strings.Contains(header, "Alasan Bergabung"), strings.Contains(header, "Ide Inovasi"), strings.Contains(header, "Pengalaman"), header == "Aplikasi Desain", header == "Perusahaan yang Terhubung":
		return 48
	default:
		return 22
	}
}
