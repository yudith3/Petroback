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

type departemenSeeder struct{}

func NewDepartemenSeeder() *departemenSeeder {
	return &departemenSeeder{}
}

func (s *departemenSeeder) Seeder(db *gorm.DB) error {
	slog.Info("Seeding subdivisi...")
	jsonFile, err := os.Open("./db/seeder/data/json/departemen.json")
	if err != nil {
		return err
	}
	defer jsonFile.Close()

	jsonData, _ := io.ReadAll(jsonFile)

	var listDepartemen []entity.Departemen
	if err := json.Unmarshal(jsonData, &listDepartemen); err != nil {
		return err
	}

	if !db.Migrator().HasTable(&entity.Departemen{}) {
		if err := db.Migrator().CreateTable(&entity.Departemen{}); err != nil {
			return err
		}
	}

	for _, data := range listDepartemen {
		var departemen entity.Departemen
		err := db.Where("id = ?", data.ID).First(&departemen).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := db.Create(&data).Error; err != nil {
				return err
			}
		}
	}

	slog.Info("Seeding subdivisi...done")
	return nil
}
