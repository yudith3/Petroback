package entity

import (
	"github.com/google/uuid"
)

type UserRole string

const (
	RoleUser          UserRole = "user"
	RoleModerator     UserRole = "moderator"
	RoleSuperAdmin    UserRole = "superadmin"
	RoleDivisionAdmin UserRole = "division_admin"
)

// role disimpan sebagai varchar dan bukan ENUM
// jadi logic status merupakan tanggung jawab service
func (r UserRole) IsValid() bool {
	switch r {
	case RoleUser, RoleModerator, RoleSuperAdmin, RoleDivisionAdmin:
		return true
	default:
		return false
	}
}

type User struct {
	ID         uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name       string     `gorm:"type:varchar(150);not null" json:"name"`
	Email      string     `gorm:"type:varchar(255);not null;unique" json:"email"`
	Password   string     `gorm:"type:varchar(255);not null" json:"-"`
	Instansi   string     `gorm:"type:varchar(150);not null" json:"instansi"`
	NoTelp     string     `gorm:"type:varchar(30);not null" json:"no_telp"`
	Role       UserRole   `gorm:"type:varchar(20);not null;default:'user'" json:"role"`
	DivisiID   *uuid.UUID `gorm:"type:uuid;index" json:"division_id,omitempty"`
	IsVerified bool       `gorm:"type:boolean;not null;default:false" json:"is_verified"`

	Timestamp Timestamp `gorm:"embedded"`

	Divisi *Divisi `gorm:"foreignKey:DivisiID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"division,omitempty"`
}

func (User) TableName() string {
	return "users"
}
