package entity

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestModelsHaveValidSchemas(t *testing.T) {
	models := []struct {
		name  string
		value any
	}{
		{"User", &User{}},
		{"Fakultas", &Fakultas{}},
		{"Departemen", &Departemen{}},
		{"Divisi", &Divisi{}},
		{"OprecStaff", &OprecStaff{}},
		{"OprecStaffDocument", &OprecStaffDocument{}},
		{"OprecStaffChoice", &OprecStaffChoice{}},
		{"OprecStaffSkill", &OprecStaffSkill{}},
		{"OprecStaffAdditionalInformation", &OprecStaffAdditionalInformation{}},
		{"OprecStaffStatusAudit", &OprecStaffStatusAudit{}},
		{"InterviewResult", &InterviewResult{}},
		{"ShortLink", &ShortLink{}},
	}

	for _, model := range models {
		t.Run(model.name, func(t *testing.T) {
			parsed := parseSchema(t, model.value)
			for _, column := range []string{"created_at", "updated_at"} {
				if parsed.FieldsByDBName[column] == nil {
					t.Errorf("schema is missing timestamp column %q", column)
				}
			}
			if model.name != "User" && parsed.FieldsByDBName["deleted_at"] == nil {
				t.Error("schema is missing soft-delete column")
			}
		})
	}
}

func TestShortLinkSlugIsUnique(t *testing.T) {
	parsed := parseSchema(t, &ShortLink{})
	for _, index := range parsed.ParseIndexes() {
		if index.Class == "UNIQUE" && len(index.Fields) == 1 && index.Fields[0].DBName == "slug" {
			return
		}
	}
	t.Fatal("short link schema has no unique slug index")
}

func TestOprecStaffChoiceHasUniqueChoiceSlots(t *testing.T) {
	parsed := parseSchema(t, &OprecStaffChoice{})
	for _, index := range parsed.ParseIndexes() {
		if index.Class == "UNIQUE" && len(index.Fields) == 2 &&
			index.Fields[0].DBName == "oprec_staff_id" && index.Fields[1].DBName == "pilihan" {
			return
		}
	}
	t.Fatal("schema has no unique index for each applicant's choice slot")
}

func TestOprecStaffChoiceLimitsChoiceNumbers(t *testing.T) {
	parsed := parseSchema(t, &OprecStaffChoice{})
	for _, check := range parsed.ParseCheckConstraints() {
		if check.Field.DBName == "pilihan" && check.Constraint == "pilihan IN (1, 2)" {
			return
		}
	}
	t.Fatal("schema has no check limiting choices to 1 and 2")
}

func TestOprecStaffAssignmentsReferenceSubdivisi(t *testing.T) {
	parsed := parseSchema(t, &OprecStaff{})
	for _, name := range []string{"AcceptedDivisi", "CurrentSubdivision"} {
		relation := parsed.Relationships.Relations[name]
		if relation == nil {
			t.Fatalf("schema is missing %q relation", name)
		}
		if relation.FieldSchema.Table != (Subdivisi{}).TableName() {
			t.Errorf("%s references table %q, want %q", name, relation.FieldSchema.Table, (Subdivisi{}).TableName())
		}
	}
}

func TestOprecStaffScreeningStatusesAreValid(t *testing.T) {
	for _, status := range []OprecStaffStatus{
		OprecStaffStatusScreeningPrimary,
		OprecStaffStatusScreeningSecondary,
	} {
		if !status.IsValid() {
			t.Errorf("screening status %q is invalid", status)
		}
	}
}

func TestOprecStaffSkillLevelValues(t *testing.T) {
	valid := []OprecStaffSkillLevel{
		OprecStaffSkillLevelNone,
		OprecStaffSkillLevelPoor,
		OprecStaffSkillLevelDecent,
		OprecStaffSkillLevelGood,
		OprecStaffSkillLevelVeryGood,
	}
	for _, level := range valid {
		if !level.IsValid() {
			t.Errorf("skill level %q is invalid", level)
		}
	}
	if OprecStaffSkillLevel("expert").IsValid() {
		t.Fatal("unsupported skill level was accepted")
	}
}

func TestAdminRoleValues(t *testing.T) {
	for _, role := range []UserRole{RoleSuperAdmin, RoleDivisionAdmin} {
		if !role.IsValid() {
			t.Errorf("admin role %q is invalid", role)
		}
	}
	if UserRole("admin").IsValid() {
		t.Fatal("legacy admin role should be migrated instead of accepted")
	}
}

func parseSchema(t *testing.T, model any) *schema.Schema {
	t.Helper()
	parsed, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("parse model schema: %v", err)
	}
	return parsed
}
