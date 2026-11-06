package seeds

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"gorm.io/gorm"
)

type subdivisiSeeder struct{}

var retiredSubdivisionIDs = []string{
	"b3e5a7d1-1c4d-4e9f-8a3c-5d1e8b2c4d7f", // Plan of Development
	"f5c1d8a3-2b4c-4d7e-9f8a-5c1d7e8b9f6d", // Oil Rig Design Competition
}

var retiredSubdivisionNames = []string{
	"Plan of Development",
	"Oil Rig Design",
	"Oil Rig Design Competition",
	"City Tour",
}

var exactSubdivisionIDsByDivision = map[string][]string{
	"7d4e1b9c-3f5a-4c8d-9e2b-1a4c7d8e5f3a": {
		"d2f6a4e1-8b3c-4a9d-1e4f-6a2d9c8f5e3d", // Marketing
		"b9d4e2c8-1f6a-4e7d-8c2b-4d9e6f8a5c1b", // External Relations
	},
	"6383bbc6-ba6a-4094-961b-e9c892be42da": {
		"a8c3f1d7-2e5b-4d6c-9f1a-3c8d5e7b4a9f", // Creative Design
		"f4b8c6a3-1d5e-4c2f-3a6b-8c4f2e1b7a5f", // Media Production (Field)
		"e3a7b5f2-9c4d-4b1e-2f5a-7b3e1d9a6f4e", // Media Production (Editing)
		"c1e5f3d9-7a2b-4f8c-9d3e-5f1c8a7e4d2c", // Experimental Design
	},
}

func NewSubdivisiSeeder() *subdivisiSeeder {
	return &subdivisiSeeder{}
}

func (s *subdivisiSeeder) Seeder(db *gorm.DB) error {
	slog.Info("Seeding subdivisi...")
	jsonFile, err := os.Open("./db/seeder/data/json/subdivisi.json")
	if err != nil {
		return err
	}
	defer jsonFile.Close()

	jsonData, _ := io.ReadAll(jsonFile)

	var listSubdivisi []entity.Subdivisi
	if err := json.Unmarshal(jsonData, &listSubdivisi); err != nil {
		return err
	}

	if !db.Migrator().HasTable(&entity.Subdivisi{}) {
		if err := db.Migrator().CreateTable(&entity.Subdivisi{}); err != nil {
			return err
		}
	}

	for _, data := range listSubdivisi {
		var subdivisi entity.Subdivisi
		err := db.Where("id = ?", data.ID).First(&subdivisi).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			data.IsActive = true
			if err := db.Create(&data).Error; err != nil {
				return err
			}
			continue
		}
		if err := db.Model(&subdivisi).Updates(map[string]interface{}{
			"divisi_id": data.DivisiID,
			"nama":      data.Nama,
			"deskripsi": data.Deskripsi,
			"is_active": true,
		}).Error; err != nil {
			return err
		}
	}

	if err := db.Model(&entity.Subdivisi{}).
		Where("id IN ? OR nama IN ?", retiredSubdivisionIDs, retiredSubdivisionNames).
		Update("is_active", false).Error; err != nil {
		return err
	}

	for divisionID, activeSubdivisionIDs := range exactSubdivisionIDsByDivision {
		if err := db.Model(&entity.Subdivisi{}).
			Where("divisi_id = ? AND id NOT IN ?", divisionID, activeSubdivisionIDs).
			Update("is_active", false).Error; err != nil {
			return err
		}
	}

	slog.Info("Seeding subdivisi...done")
	return nil
}
