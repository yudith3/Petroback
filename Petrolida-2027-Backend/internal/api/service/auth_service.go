package service

import (
	"errors"
	"strings"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/repository"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService interface {
	ProvisionAdmin() error
	Login(email, password string) (string, domain.AdminProfile, error)
	Authenticate(token string) (domain.AdminProfile, error)
}

type authService struct {
	repository repository.AuthRepository
	config     config.AuthConfig
	now        func() time.Time
}

type sessionClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func NewAuth(repository repository.AuthRepository, cfg config.AuthConfig) AuthService {
	return &authService{repository: repository, config: cfg, now: time.Now}
}

func (s *authService) ProvisionAdmin() error {
	if s.config.AdminEmail == "" && s.config.AdminPassword == "" {
		return nil
	}
	if s.config.AdminEmail == "" || s.config.AdminPassword == "" || s.config.AdminName == "" {
		return errors.New("ADMIN_NAME, ADMIN_EMAIL, and ADMIN_PASSWORD must be set together")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(s.config.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repository.UpsertSuperAdmin(entity.User{
		Name: s.config.AdminName, Email: strings.ToLower(strings.TrimSpace(s.config.AdminEmail)),
		Password: string(hash), Instansi: s.config.AdminInstansi, NoTelp: s.config.AdminNoTelepon,
		Role: entity.RoleSuperAdmin, IsVerified: true,
	})
}

func (s *authService) Login(email, password string) (string, domain.AdminProfile, error) {
	user, err := s.repository.FindByEmail(strings.ToLower(strings.TrimSpace(email)))
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		return "", domain.AdminProfile{}, domain.ErrInvalidCredentials
	}
	if user.Role != entity.RoleSuperAdmin && user.Role != entity.RoleDivisionAdmin {
		return "", domain.AdminProfile{}, domain.ErrForbidden
	}
	if user.Role == entity.RoleDivisionAdmin && (user.DivisiID == nil || user.Divisi == nil || !user.Divisi.IsActive) {
		return "", domain.AdminProfile{}, domain.ErrForbidden
	}
	now := s.now()
	claims := sessionClaims{
		Role: string(user.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: user.ID.String(), Issuer: "Petrolida 2027", IssuedAt: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.config.SessionTTL)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		return "", domain.AdminProfile{}, err
	}
	return token, adminProfile(user), nil
}

func (s *authService) Authenticate(tokenString string) (domain.AdminProfile, error) {
	claims := &sessionClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, domain.ErrUnauthenticated
		}
		return []byte(s.config.JWTSecret), nil
	})
	if err != nil || !token.Valid || claims.Subject == "" {
		return domain.AdminProfile{}, domain.ErrUnauthenticated
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return domain.AdminProfile{}, domain.ErrUnauthenticated
	}
	user, err := s.repository.FindByID(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.AdminProfile{}, domain.ErrUnauthenticated
	}
	if err != nil {
		return domain.AdminProfile{}, err
	}
	if (user.Role != entity.RoleSuperAdmin && user.Role != entity.RoleDivisionAdmin) || claims.Role != string(user.Role) {
		return domain.AdminProfile{}, domain.ErrForbidden
	}
	if user.Role == entity.RoleDivisionAdmin && (user.DivisiID == nil || user.Divisi == nil || !user.Divisi.IsActive) {
		return domain.AdminProfile{}, domain.ErrForbidden
	}
	return adminProfile(user), nil
}

func adminProfile(user entity.User) domain.AdminProfile {
	profile := domain.AdminProfile{ID: user.ID, Name: user.Name, Email: user.Email, Role: string(user.Role)}
	if user.Divisi != nil {
		profile.Division = &domain.MasterReference{ID: user.Divisi.ID, Name: user.Divisi.Nama}
	}
	return profile
}
