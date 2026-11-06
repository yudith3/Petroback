package export

import (
	"bytes"
	"encoding/csv"
	"slices"
	"strings"
	"testing"
)

func readCSV(t *testing.T, table Table) [][]string {
	t.Helper()
	var buffer bytes.Buffer
	if err := WriteCSV(&buffer, table); err != nil {
		t.Fatalf("WriteCSV() error = %v", err)
	}
	content, found := strings.CutPrefix(buffer.String(), utf8BOM)
	if !found {
		t.Fatal("CSV does not start with UTF-8 BOM")
	}
	records, err := csv.NewReader(strings.NewReader(content)).ReadAll()
	if err != nil {
		t.Fatalf("read CSV: %v", err)
	}
	return records
}

func TestWriteCSVEmptyTableWritesHeaderOnly(t *testing.T) {
	records := readCSV(t, Table{Columns: []Column{{Header: "Nama"}, {Header: "NRP", Text: true}}})
	if len(records) != 1 || !slices.Equal(records[0], []string{"Nama", "NRP"}) {
		t.Fatalf("records = %v, want header only", records)
	}
}

func TestWriteCSVKeepsSpecialCharacters(t *testing.T) {
	values := []string{"Budi, S.T.", "baris 1\nbaris 2", `kata "kutip"`, "Zoë 🚀 石油"}
	records := readCSV(t, Table{
		Columns: []Column{{Header: "A"}, {Header: "B"}, {Header: "C"}, {Header: "D"}},
		Rows:    [][]string{values},
	})
	if len(records) != 2 || !slices.Equal(records[1], values) {
		t.Fatalf("row = %q, want %q", records[1], values)
	}
}

func TestWriteCSVNeutralizesFormula(t *testing.T) {
	for _, value := range []string{"=1+1", "+62", "-2", "@SUM(A1)", "\t=1", "\r=1", `=HYPERLINK("http://x")`} {
		t.Run(value, func(t *testing.T) {
			records := readCSV(t, Table{Columns: []Column{{Header: "A"}}, Rows: [][]string{{value}}})
			if got, want := records[1][0], "'"+value; got != want {
				t.Fatalf("cell = %q, want %q", got, want)
			}
		})
	}
}

func TestWriteCSVNeutralizesFormulaHeader(t *testing.T) {
	records := readCSV(t, Table{Columns: []Column{{Header: "=cmd"}}})
	if got, want := records[0][0], "'=cmd"; got != want {
		t.Fatalf("header = %q, want %q", got, want)
	}
}

func TestWriteCSVKeepsTextColumnsAsText(t *testing.T) {
	tests := map[string]string{
		"081234567890":   `="081234567890"`,
		"+6281234567890": `="+6281234567890"`,
		"5025251178":     `="5025251178"`,
		"":               "",
		`=1+"2"`:         `'=1+"2"`,
		"08 123":         "08 123",
	}
	for value, want := range tests {
		records := readCSV(t, Table{
			Columns: []Column{{Header: "NRP", Text: true}, {Header: "Nama"}},
			Rows:    [][]string{{value, "Budi"}},
		})
		if got := records[1][0]; got != want {
			t.Errorf("text cell %q = %q, want %q", value, got, want)
		}
	}
}

func TestWriteCSVPadsShortRows(t *testing.T) {
	records := readCSV(t, Table{Columns: []Column{{Header: "A"}, {Header: "B"}}, Rows: [][]string{{"x"}}})
	if !slices.Equal(records[1], []string{"x", ""}) {
		t.Fatalf("row = %q, want padded row", records[1])
	}
}
