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

type divisiSeeder struct{}

func NewDivisiSeeder() *divisiSeeder {
	return &divisiSeeder{}
}

func (s *divisiSeeder) Seeder(db *gorm.DB) error {
	slog.Info("Seeding divisi...")
	jsonFile, err := os.Open("./db/seeder/data/json/divisi.json")
	if err != nil {
		return err
	}
	defer jsonFile.Close()

	jsonData, _ := io.ReadAll(jsonFile)

	var listDivisi []entity.Divisi
	if err := json.Unmarshal(jsonData, &listDivisi); err != nil {
		return err
	}

	if !db.Migrator().HasTable(&entity.Divisi{}) {
		if err := db.Migrator().CreateTable(&entity.Divisi{}); err != nil {
			return err
		}
	}

	for _, data := range listDivisi {
		var divisi entity.Divisi
		err := db.Where("id = ?", data.ID).First(&divisi).Error
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
		if err := db.Model(&divisi).Updates(map[string]interface{}{
			"nama":      data.Nama,
			"deskripsi": data.Deskripsi,
			"is_active": true,
		}).Error; err != nil {
			return err
		}
	}

	slog.Info("Seeding divisi...done")
	return nil
}
