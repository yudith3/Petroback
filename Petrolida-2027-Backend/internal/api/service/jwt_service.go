package service

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	MyError "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	"github.com/golang-jwt/jwt/v4"
)

type (
	JWTService interface {
		GenerateToken(userId string, role string) string
		ValidateToken(token string) (*jwt.Token, error)
		GetPayloadInsideToken(token string) (string, string, error)
		//GenerateGeneralPurposeToken(data map[string]any, expired time.Duration) (string, error)
		//GetPayloadInsideGeneralPurposeToken(token string) (jwt.MapClaims, error)
	}

	jwtCustomClaim struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
		jwt.RegisteredClaims
	}

	jwtService struct {
		secretKey string
		issuer    string
	}
)

func NewJWTService() JWTService {
	return &jwtService{
		secretKey: getSecretKey(),
		issuer:    "PETROLIDA 2027",
	}
}

func getSecretKey() string {
	secretKey := os.Getenv("JWT_SECRET")
	if secretKey == "" {
		secretKey = "Template"
	}
	return secretKey
}

func (j *jwtService) GenerateToken(userId string, role string) string {
	claims := jwtCustomClaim{
		userId,
		role,
		jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute * time.Duration(domain.JWT_EXPIRE_TIME_IN_MINUTES))),
			Issuer:    j.issuer,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tx, err := token.SignedString([]byte(j.secretKey))
	if err != nil {
		log.Println(err)
	}
	return tx
}

func (j *jwtService) ValidateToken(token string) (*jwt.Token, error) {
	return jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, MyError.New(MyError.TypeAuthorization, "UNEXPECTED_SIGNING_METHOD", fmt.Sprintf("unexpected signing method %v", token.Header["alg"]))
		}
		return []byte(j.secretKey), nil
	})
}

func (j *jwtService) GetPayloadInsideToken(token string) (string, string, error) {
	t_Token, err := j.ValidateToken(token)
	if err != nil {
		return "", "", domain.ErrTokenExpired
	}

	if !t_Token.Valid {
		return "", "", domain.ErrTokenInvalid
	}

	claims := t_Token.Claims.(jwt.MapClaims)
	id := fmt.Sprintf("%v", claims["user_id"])
	role := fmt.Sprintf("%v", claims["role"])
	return id, role, nil
}

func (j *jwtService) GenerateGeneralPurposeToken(data map[string]any, expired time.Duration) (string, error) {
	claims := jwt.MapClaims{}
	for key, value := range data {
		claims[key] = value
	}

	claims["exp"] = time.Now().Add(expired).Unix()
	claims["iat"] = time.Now().Unix()
	claims["iss"] = j.issuer

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.secretKey))
}
