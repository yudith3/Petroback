package migrations

import (
	"fmt"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func Migrate(db *gorm.DB) error {
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";").Error; err != nil {
		return fmt.Errorf("create uuid-ossp extension: %w", err)
	}
	if err := migrateLegacyOprecStaffChoices(db); err != nil {
		return err
	}
	if err := prepareAcceptedSubdivisionForeignKey(db); err != nil {
		return err
	}
	if err := db.AutoMigrate(
		&entity.Fakultas{},
		&entity.Departemen{},
		&entity.Divisi{},
		&entity.Subdivisi{},
		&entity.User{},
		&entity.OprecStaff{},
		&entity.OprecStaffDocument{},
		&entity.OprecStaffChoice{},
		&entity.OprecStaffSkill{},
		&entity.OprecStaffAdditionalInformation{},
		&entity.OprecStaffStatusAudit{},
		&entity.InterviewResult{},
		&entity.StaffSelection{},
		&entity.ShortLink{},
		&entity.PetroRunOrder{}, &entity.PetroRunParticipant{}, &entity.PetroRunPaymentProof{}, &entity.PetroRunTicket{}, &entity.PetroRunAuditLog{},
	); err != nil {
		return err
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&entity.StaffSelection{ID: entity.StaffSelectionID}).Error; err != nil {
		return fmt.Errorf("initialize staff selection lock: %w", err)
	}
	if err := db.Exec("CREATE SEQUENCE IF NOT EXISTS petro_run_order_number_seq").Error; err != nil {
		return fmt.Errorf("create PetroRun order number sequence: %w", err)
	}
	if err := db.Exec("CREATE SEQUENCE IF NOT EXISTS petro_run_ticket_number_seq").Error; err != nil {
		return fmt.Errorf("create PetroRun ticket number sequence: %w", err)
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&entity.StaffSelection{ID: entity.StaffSelectionID}).Error; err != nil {
		return fmt.Errorf("initialize staff selection lock: %w", err)
	}
	if err := migrateLegacyAdminRoles(db); err != nil {
		return err
	}
	if err := db.Exec(`UPDATE oprec_staff SET primary_rejected = TRUE WHERE status IN ('screening_secondary', 'rejected') AND primary_rejected = FALSE`).Error; err != nil {
		return fmt.Errorf("backfill primary rejection: %w", err)
	}
	if err := db.Exec(`UPDATE oprec_staff SET secondary_rejected = TRUE WHERE status = 'rejected' AND secondary_rejected = FALSE`).Error; err != nil {
		return fmt.Errorf("backfill secondary rejection: %w", err)
	}
	if err := enableRowLevelSecurity(db); err != nil {
		return err
	}

	return nil
}

func prepareAcceptedSubdivisionForeignKey(db *gorm.DB) error {
	if !db.Migrator().HasTable(&entity.OprecStaff{}) ||
		!db.Migrator().HasColumn(&entity.OprecStaff{}, "AcceptedDivisiID") ||
		!db.Migrator().HasTable(&entity.Subdivisi{}) {
		return nil
	}

	var invalidReferences int64
	if err := db.Raw(`
		SELECT COUNT(*)
		FROM oprec_staff registrants
		WHERE registrants.accepted_divisi_id IS NOT NULL
		  AND NOT EXISTS (
			SELECT 1 FROM subdivisi subdivisions
			WHERE subdivisions.id = registrants.accepted_divisi_id
		  )
	`).Scan(&invalidReferences).Error; err != nil {
		return fmt.Errorf("check accepted subdivision references: %w", err)
	}
	if invalidReferences > 0 {
		return fmt.Errorf(
			"cannot migrate accepted_divisi_id: %d existing value(s) do not reference subdivisi; map them to subdivisions before restarting",
			invalidReferences,
		)
	}

	if err := db.Exec(`
		ALTER TABLE oprec_staff
		DROP CONSTRAINT IF EXISTS fk_oprec_staff_accepted_divisi
	`).Error; err != nil {
		return fmt.Errorf("drop legacy accepted division foreign key: %w", err)
	}
	return nil
}

func migrateLegacyAdminRoles(db *gorm.DB) error {
	const superAdminEmail = "admin@petrolida.com"
	if err := db.Exec(
		`UPDATE users SET role = ?, divisi_id = NULL WHERE role = ? AND LOWER(email) = LOWER(?)`,
		entity.RoleSuperAdmin,
		"admin",
		superAdminEmail,
	).Error; err != nil {
		return fmt.Errorf("migrate legacy admin roles: %w", err)
	}
	if err := db.Exec(`
		WITH division_accounts (email, division_name) AS (
			VALUES
				('fnc@petrolida.com', 'Finance'),
				('crtv@petrolida.com', 'Creative'),
				('damn@petrolida.com', 'Data Management'),
				('dam@petrolida.com', 'Data Management'),
				('opr@petrolida.com', 'Operational'),
				('ncomp@petrolida.com', 'Non Competition'),
				('comp@petrolida.com', 'Competition'),
				('hc@petrolida.com', 'Human Capital'),
				('pr@petrolida.com', 'Public Relations')
		)
		UPDATE users AS users
		SET role = ?, divisi_id = divisions.id
		FROM division_accounts AS accounts
		JOIN divisi AS divisions
			ON LOWER(divisions.nama) = LOWER(accounts.division_name)
		WHERE LOWER(users.email) = accounts.email
		  AND (
			users.role = 'admin'
			OR (users.role = ? AND users.divisi_id IS NULL)
		  )
	`, entity.RoleDivisionAdmin, entity.RoleDivisionAdmin).Error; err != nil {
		return fmt.Errorf("map legacy division admin accounts: %w", err)
	}
	if err := db.Exec(`
		UPDATE users AS users
		SET role = ?, divisi_id = divisions.id
		FROM divisi AS divisions
		WHERE users.role = 'admin'
		  AND LOWER(users.email) <> LOWER(?)
		  AND (
			LOWER(TRIM(users.name)) = LOWER(TRIM(divisions.nama))
			OR (
				LOWER(TRIM(users.name)) = 'public relation'
				AND LOWER(TRIM(divisions.nama)) = 'public relations'
			)
		  )
	`, entity.RoleDivisionAdmin, superAdminEmail).Error; err != nil {
		return fmt.Errorf("match legacy division admins by name: %w", err)
	}
	if err := db.Exec(
		`UPDATE users SET role = ?, divisi_id = NULL WHERE role = ? AND LOWER(email) <> LOWER(?)`,
		entity.RoleUser,
		entity.RoleSuperAdmin,
		superAdminEmail,
	).Error; err != nil {
		return fmt.Errorf("enforce canonical superadmin: %w", err)
	}
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_users_single_active_superadmin
		ON users (role)
		WHERE role = 'superadmin' AND deleted_at IS NULL
	`).Error; err != nil {
		return fmt.Errorf("create single superadmin index: %w", err)
	}
	return nil
}

// enableRowLevelSecurity prevents Supabase's anon and authenticated Data API
// roles from accessing application tables without an explicit policy. The API
// connects directly to PostgreSQL as the table owner, so its existing queries
// continue to work while the public Data API is closed by default.
func enableRowLevelSecurity(db *gorm.DB) error {
	tables := []string{
		(entity.User{}).TableName(),
		(entity.Fakultas{}).TableName(),
		(entity.Departemen{}).TableName(),
		(entity.Divisi{}).TableName(),
		(entity.Subdivisi{}).TableName(),
		(entity.OprecStaff{}).TableName(),
		(entity.OprecStaffDocument{}).TableName(),
		(entity.OprecStaffChoice{}).TableName(),
		(entity.OprecStaffSkill{}).TableName(),
		(entity.OprecStaffAdditionalInformation{}).TableName(),
		(entity.OprecStaffStatusAudit{}).TableName(),
		(entity.InterviewResult{}).TableName(),
		(entity.StaffSelection{}).TableName(),
		(entity.ShortLink{}).TableName(),
		(entity.PetroRunOrder{}).TableName(),
		(entity.PetroRunParticipant{}).TableName(),
		(entity.PetroRunPaymentProof{}).TableName(),
		(entity.PetroRunTicket{}).TableName(),
		(entity.PetroRunAuditLog{}).TableName(),
	}

	for _, table := range tables {
		statement := fmt.Sprintf(`ALTER TABLE %q ENABLE ROW LEVEL SECURITY`, table)
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("enable row level security on %s: %w", table, err)
		}
	}

	return nil
}

// migrateLegacyOprecStaffChoices keeps databases created by the previous
// division-based recruitment model writable. The current model stores a
// subdivision_id, but AutoMigrate intentionally does not remove the legacy
// divisi_id column. Leaving that column NOT NULL makes every new registration
// fail even though divisi_id is no longer part of the application model.
//
// The column is retained (and only made nullable) so historical data is never
// discarded by an automatic startup migration.
func migrateLegacyOprecStaffChoices(db *gorm.DB) error {
	const statement = `
DO $$
BEGIN
	IF EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = 'oprec_staff_choices'
		  AND column_name = 'divisi_id'
	) THEN
		ALTER TABLE oprec_staff_choices
			ALTER COLUMN divisi_id DROP NOT NULL;
	END IF;
END $$;`

	if err := db.Exec(statement).Error; err != nil {
		return fmt.Errorf("migrate legacy oprec staff choices: %w", err)
	}
	return nil
}
