package seeds

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestApprovedStaffRoster(t *testing.T) {
	file, err := os.Open("../data/json/staff_results.json")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	results, err := loadStaffResults(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 221 {
		t.Fatalf("staff count = %d, want 221", len(results))
	}
	masterData, err := os.ReadFile("../data/json/subdivisi.json")
	if err != nil {
		t.Fatal(err)
	}
	var subdivisions []struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	}
	if err := json.Unmarshal(masterData, &subdivisions); err != nil {
		t.Fatal(err)
	}
	names := map[uuid.UUID]string{}
	for _, sub := range subdivisions {
		names[sub.ID] = sub.Name
	}
	wantCounts := map[string]int{
		"Administration": 8, "Web Development - Frontend": 7,
		"Web Development - Backend": 5, "Web Development - UI/UX": 6,
		"Sponsorship": 7, "Fundraise": 7, "Paper & Poster Competition": 5,
		"Petrosmart Competition": 4, "Business Case Competition": 6,
		"Case Study Competition": 6, "Fracturing Fluid Design Competition": 5,
		"Geothermal Case Competition": 6, "Petrovis": 5,
		"Petrolida Benchmarking Management": 6, "PetroRun": 8, "Gala Dinner": 6,
		"Logistic": 18, "Security and License": 15, "Consumption": 7,
		"Accomodation and Transportation": 15, "Sub Coor LO": 8,
		"Marketing": 11, "External Relations": 6, "Media Production (Field)": 7,
		"Media Production (Editing)": 4, "Creative Design": 11,
		"Experimental Design": 10, "Internalization": 6, "People & Development": 6,
	}
	counts := map[string]int{}
	wantNames := map[string]string{
		"5025251273": "Cokorda Bagus Laksmana Iswara",
		"5019261078": "Nisya Dwi Andani",
		"5017241026": "Mirza Anas Fauzi",
		"5007261016": "Atala Keano Bovita",
		"5017251042": "Keysha Fiorella",
		"5022261024": "Arifin Hanif",
		"2042261122": "Rakha Alauddin Arifin",
		"5030251031": "Salma Zidna Al Ka'bati",
		"5029251019": "Kenisha Fatihah Azzahra",
	}
	for _, result := range results {
		name, known := names[result.SubdivisionID]
		if !known {
			t.Fatalf("unknown subdivision for %s: %s", result.NRP, result.SubdivisionID)
		}
		counts[name]++
		if expected, check := wantNames[result.NRP]; check {
			if result.Name != expected {
				t.Errorf("name for %s = %q, want %q", result.NRP, result.Name, expected)
			}
			delete(wantNames, result.NRP)
		}
	}
	if len(wantNames) != 0 {
		t.Errorf("missing staff: %v", wantNames)
	}
	if len(counts) != len(wantCounts) {
		t.Fatalf("subdivision count = %d, want %d", len(counts), len(wantCounts))
	}
	for name, expected := range wantCounts {
		if counts[name] != expected {
			t.Errorf("%s count = %d, want %d", name, counts[name], expected)
		}
	}
}

func TestStaffRosterRejectsInvalidOrAmbiguousData(t *testing.T) {
	valid := `{"nrp":"5025251273","name":"Applicant","subdivision_id":"a91c0114-9377-403d-8838-8200bc24d9e2"}`
	for name, input := range map[string]string{
		"empty": `[]`, "malformed": `{`, "null": `null`,
		"invalid ID":         strings.Replace("["+valid+"]", "5025251273", "502525127", 1),
		"missing name":       strings.Replace("["+valid+"]", "Applicant", " ", 1),
		"missing assignment": `[{"nrp":"5025251273","name":"Applicant"}]`,
		"invalid assignment": strings.Replace("["+valid+"]", "a91c0114-9377-403d-8838-8200bc24d9e2", "invalid", 1),
		"duplicate ID":       "[" + valid + "," + valid + "]",
		"trailing data":      "[" + valid + "] []",
		"misspelled field":   strings.Replace("["+valid+"]", "subdivision_id", "subdivison_id", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := loadStaffResults(strings.NewReader(input)); err == nil {
				t.Fatal("invalid roster was accepted")
			}
		})
	}
	input := strings.ReplaceAll(strings.ReplaceAll("["+valid+"]", "5025251273", " 5025251273 "), "Applicant", " Applicant ")
	results, err := loadStaffResults(strings.NewReader(input))
	if err != nil || len(results) != 1 || results[0].NRP != "5025251273" || results[0].Name != "Applicant" {
		t.Fatalf("trimmed roster = %+v, error = %v", results, err)
	}
}
