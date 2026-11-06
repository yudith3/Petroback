package seeds

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestApprovedInterviewRoster(t *testing.T) {
	file, err := os.Open("../data/json/interview_results.json")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	results, err := loadInterviewResults(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 390 {
		t.Fatalf("unique results = %d, want 390", len(results))
	}
	masterData, err := os.ReadFile("../data/json/subdivisi.json")
	if err != nil {
		t.Fatal(err)
	}
	var subdivisions []struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal(masterData, &subdivisions); err != nil {
		t.Fatal(err)
	}
	knownSubdivisions := map[uuid.UUID]bool{}
	for _, sub := range subdivisions {
		knownSubdivisions[sub.ID] = true
	}
	held := map[string]bool{"5019261031": false, "5046251070": false, "5061261003": false, "5033251113": false, "5026261032": false}
	corrected := map[string]bool{"5011251134": false, "5056251049": false, "351518190408001": false}
	accepted := 0
	for _, result := range results {
		if _, exists := held[result.NRP]; exists {
			if result.Status != "on_hold" || result.SubdivisionID != nil {
				t.Fatalf("duplicate not held: %+v", result)
			}
			held[result.NRP] = true
		} else {
			if result.Status != "accepted" {
				t.Fatalf("unique applicant not accepted: %+v", result)
			}
			accepted++
			if result.NRP != "351518190408001" && (result.Name == "" || result.SubdivisionID == nil || !knownSubdivisions[*result.SubdivisionID]) {
				t.Fatalf("missing approved identity: %+v", result)
			}
		}
		if _, exists := corrected[result.NRP]; exists {
			corrected[result.NRP] = true
		}
		if result.NRP == "501251134" || result.NRP == "505625049" {
			t.Fatalf("uncorrected ID: %s", result.NRP)
		}
	}
	if accepted != 385 {
		t.Fatalf("accepted = %d, want 385", accepted)
	}
	for nrp, present := range held {
		if !present {
			t.Errorf("held ID missing: %s", nrp)
		}
	}
	for nrp, present := range corrected {
		if !present {
			t.Errorf("corrected/retained ID missing: %s", nrp)
		}
	}
}

func TestInterviewRosterRejectsIncompleteOrInvalidData(t *testing.T) {
	for _, input := range []string{`[]`, `{`, `[{"nrp":"501251134","name":"Applicant"}]`, `[{"nrp":"5011251134","name":"Applicant"}]`, `[{"nrp":"351518190408002"}]`} {
		if _, err := loadInterviewResults(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted malformed roster %s", input)
		}
	}
}
