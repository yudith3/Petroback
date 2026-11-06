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

type fakultasSeeder struct{}

func NewFakultasSeeder() *fakultasSeeder {
	return &fakultasSeeder{}
}

func (s *fakultasSeeder) Seeder(db *gorm.DB) error {
	slog.Info("Seeding fakultas...")
	jsonFile, err := os.Open("./db/seeder/data/json/fakultas.json")
	if err != nil {
		return err
	}
	defer jsonFile.Close()

	jsonData, _ := io.ReadAll(jsonFile)

	var listFakultas []entity.Fakultas
	if err := json.Unmarshal(jsonData, &listFakultas); err != nil {
		return err
	}

	if !db.Migrator().HasTable(&entity.Fakultas{}) {
		if err := db.Migrator().CreateTable(&entity.Fakultas{}); err != nil {
			return err
		}
	}

	for _, data := range listFakultas {
		var fakultas entity.Fakultas
		err := db.Where("id = ?", data.ID).First(&fakultas).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := db.Create(&data).Error; err != nil {
				return err
			}
		}
	}

	slog.Info("Seeding fakultas...done")
	return nil
}
