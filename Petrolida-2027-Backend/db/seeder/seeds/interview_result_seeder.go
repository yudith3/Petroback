package seeds

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type interviewResultSeeder struct{}

func NewInterviewResultSeeder() *interviewResultSeeder { return &interviewResultSeeder{} }

var interviewSeedNRPPattern = regexp.MustCompile(`^([0-9]{10}|351518190408001)$`)

type interviewResultSeed struct {
	NRP           string     `json:"nrp"`
	Name          string     `json:"name"`
	SubdivisionID *uuid.UUID `json:"subdivision_id"`
}

func loadInterviewResults(reader io.Reader) ([]entity.InterviewResult, error) {
	var roster []interviewResultSeed
	if err := json.NewDecoder(reader).Decode(&roster); err != nil {
		return nil, fmt.Errorf("decode interview roster: %w", err)
	}
	if len(roster) == 0 {
		return nil, fmt.Errorf("interview roster must not be empty")
	}
	byNRP := make(map[string]entity.InterviewResult, len(roster))
	for _, row := range roster {
		nrp := strings.TrimSpace(row.NRP)
		name := strings.TrimSpace(row.Name)
		if !interviewSeedNRPPattern.MatchString(nrp) {
			return nil, fmt.Errorf("invalid interview roster Student ID %q", nrp)
		}
		// Only the explicitly retained ID is allowed to omit identity metadata.
		if nrp != "351518190408001" && (name == "" || row.SubdivisionID == nil || *row.SubdivisionID == uuid.Nil) {
			return nil, fmt.Errorf("missing name or subdivision for Student ID %s", nrp)
		}
		if existing, duplicate := byNRP[nrp]; duplicate {
			existing.Status = "on_hold"
			existing.SubdivisionID = nil
			byNRP[nrp] = existing
			continue
		}
		byNRP[nrp] = entity.InterviewResult{NRP: nrp, Name: name, Status: "accepted", SubdivisionID: row.SubdivisionID}
	}
	results := make([]entity.InterviewResult, 0, len(byNRP))
	for _, result := range byNRP {
		results = append(results, result)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].NRP < results[j].NRP })
	return results, nil
}

func (s *interviewResultSeeder) Seeder(db *gorm.DB) error {
	file, err := os.Open("./db/seeder/data/json/interview_results.json")
	if err != nil {
		return err
	}
	defer file.Close()
	results, err := loadInterviewResults(file)
	if err != nil {
		return err
	}
	// Insert once so confirmed corrections in the database survive restarts.
	// This never modifies the registrant's live staff-selection decision.
	return db.Transaction(func(tx *gorm.DB) error {
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "nrp"}}, DoNothing: true}).CreateInBatches(results, 100).Error
	})
}
