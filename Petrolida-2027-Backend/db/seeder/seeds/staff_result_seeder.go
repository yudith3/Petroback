package seeds

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type staffResultSeeder struct{}

func NewStaffResultSeeder() *staffResultSeeder { return &staffResultSeeder{} }

var staffSeedNRPPattern = regexp.MustCompile(`^[0-9]{10}$`)

type staffResultSeed struct {
	NRP           string    `json:"nrp"`
	Name          string    `json:"name"`
	SubdivisionID uuid.UUID `json:"subdivision_id"`
}

func loadStaffResults(reader io.Reader) ([]staffResultSeed, error) {
	var results []staffResultSeed
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&results); err != nil {
		return nil, fmt.Errorf("decode staff roster: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("staff roster must contain a single JSON array")
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("staff roster must not be empty")
	}
	seen := make(map[string]bool, len(results))
	for i := range results {
		row := &results[i]
		row.NRP = strings.TrimSpace(row.NRP)
		row.Name = strings.TrimSpace(row.Name)
		if !staffSeedNRPPattern.MatchString(row.NRP) {
			return nil, fmt.Errorf("invalid staff roster Student ID %q", row.NRP)
		}
		if row.Name == "" || row.SubdivisionID == uuid.Nil {
			return nil, fmt.Errorf("missing name or subdivision for Student ID %s", row.NRP)
		}
		if seen[row.NRP] {
			return nil, fmt.Errorf("duplicate staff roster Student ID %s", row.NRP)
		}
		seen[row.NRP] = true
	}
	return results, nil
}

func (s *staffResultSeeder) Seeder(db *gorm.DB) error {
	file, err := os.Open("./db/seeder/data/json/staff_results.json")
	if err != nil {
		return err
	}
	defer file.Close()
	results, err := loadStaffResults(file)
	if err != nil {
		return err
	}
	return finalizeStaffResults(db, results)
}

func finalizeStaffResults(db *gorm.DB, results []staffResultSeed) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var selection entity.StaffSelection
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&selection, "id = ?", entity.StaffSelectionID).Error; err != nil {
			return err
		}
		if selection.FinalizedAt != nil {
			return nil
		}
		if len(results) == 0 {
			return fmt.Errorf("cannot finalize staff selection with an empty roster")
		}
		roster, err := json.Marshal(results)
		if err != nil {
			return err
		}
		finalizedAt := time.Now().UTC()
		var counts []struct {
			Status string
			Count  int64
		}
		// Match existing registrations by NRP. The single UPDATE preserves all
		// registration answers and names, and never creates missing applicants.
		// Rejection flags follow the existing choice-specific decision semantics.
		if err := tx.Raw(`
			WITH roster AS (
				SELECT * FROM jsonb_to_recordset(?::jsonb) AS entry(nrp text, subdivision_id uuid)
			), assignments AS (
				SELECT registrant.id, roster.subdivision_id
				FROM oprec_staff registrant
				LEFT JOIN roster ON roster.nrp = registrant.nrp
				WHERE registrant.deleted_at IS NULL
			), updated AS (
				UPDATE oprec_staff AS staff
				SET status = CASE WHEN assignments.subdivision_id IS NULL THEN 'rejected' ELSE 'accepted' END,
					accepted_divisi_id = assignments.subdivision_id,
					current_subdivision_id = NULL,
					primary_rejected = CASE
						WHEN assignments.subdivision_id IS NULL THEN TRUE
						WHEN EXISTS (SELECT 1 FROM oprec_staff_choices choice WHERE choice.oprec_staff_id = staff.id AND choice.pilihan = 1 AND choice.subdivisi_id = assignments.subdivision_id AND choice.deleted_at IS NULL) THEN FALSE
						ELSE staff.primary_rejected END,
					secondary_rejected = CASE
						WHEN assignments.subdivision_id IS NULL THEN TRUE
						WHEN EXISTS (SELECT 1 FROM oprec_staff_choices choice WHERE choice.oprec_staff_id = staff.id AND choice.pilihan = 2 AND choice.subdivisi_id = assignments.subdivision_id AND choice.deleted_at IS NULL) THEN FALSE
						ELSE staff.secondary_rejected END,
					updated_at = ?
				FROM assignments
				WHERE staff.id = assignments.id
				RETURNING staff.status
			)
			SELECT status, COUNT(*) AS count FROM updated GROUP BY status
		`, string(roster), finalizedAt).Scan(&counts).Error; err != nil {
			return fmt.Errorf("finalize staff decisions: %w", err)
		}
		for _, count := range counts {
			if count.Status == "accepted" {
				selection.AcceptedCount = count.Count
			} else {
				selection.RejectedCount = count.Count
			}
		}
		selection.FinalizedAt = &finalizedAt
		selection.UnmatchedCount = int64(len(results)) - selection.AcceptedCount
		selection.RosterHash = fmt.Sprintf("%x", sha256.Sum256(roster))
		if err := tx.Save(&selection).Error; err != nil {
			return err
		}
		slog.Info("Staff selection finalized", "accepted", selection.AcceptedCount, "rejected", selection.RejectedCount, "roster_without_registration", selection.UnmatchedCount)
		return nil
	})
}
