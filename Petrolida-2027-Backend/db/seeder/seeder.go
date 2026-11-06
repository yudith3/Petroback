package seeds

import (
	"github.com/Petrolida/Petrolida-2027-Backend/db/seeder/seeds"
	"gorm.io/gorm"
)

type Seeder interface {
	Seeder(db *gorm.DB) error
}

func RunSeeder(db *gorm.DB) error {

	// komen jika mau membatalkan seedernya
	seeders := []Seeder{
		seeds.NewFakultasSeeder(),
		seeds.NewDepartemenSeeder(),
		seeds.NewDivisiSeeder(),
		seeds.NewSubdivisiSeeder(),
		seeds.NewInterviewResultSeeder(),
		seeds.NewStaffResultSeeder(),
	}

	// if os.Getenv("APP_ENV") == "development" {
	// 	if err := db.Exec("TRUNCATE TABLE users RESTART IDENTITY CASCADE").Error; err != nil {
	// 		return err
	// 	}
	// }
	for _, seeder := range seeders {
		if err := seeder.Seeder(db); err != nil {
			return err
		}
	}

	return nil
}
