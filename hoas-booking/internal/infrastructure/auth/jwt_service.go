package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/yourusername/hoas-booking/config"
	"github.com/yourusername/hoas-booking/internal/domain/user"
)

type Claims struct {
	UserID uuid.UUID `json:"uid"`
	OrgID  uuid.UUID `json:"oid"`
	Role   user.Role `json:"role"`
	jwt.RegisteredClaims
}

type JWTService struct { cfg config.JWTConfig }

func NewJWTService(cfg config.JWTConfig) *JWTService { return &JWTService{cfg: cfg} }

func RoleFromString(r interface{}) user.Role {
	switch v := r.(type) {
	case user.Role: return v
	case string: return user.Role(v)
	default: return user.RoleTenant
	}
}

func (s *JWTService) IssueAccessToken(userID, orgID uuid.UUID, role user.Role) (string, error) {
	claims := Claims{UserID: userID, OrgID: orgID, Role: role,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.cfg.AccessTTL)), IssuedAt: jwt.NewNumericDate(time.Now()), Subject: userID.String()}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.AccessSecret))
}

func (s *JWTService) IssueRefreshToken(userID, orgID uuid.UUID, role user.Role) (string, error) {
	claims := Claims{UserID: userID, OrgID: orgID, Role: role,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.cfg.RefreshTTL)), IssuedAt: jwt.NewNumericDate(time.Now()), Subject: userID.String()}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.RefreshSecret))
}

func (s *JWTService) ValidateAccessToken(tokenStr string) (*Claims, error) { return s.validate(tokenStr, s.cfg.AccessSecret) }
func (s *JWTService) ValidateRefreshToken(tokenStr string) (*Claims, error) { return s.validate(tokenStr, s.cfg.RefreshSecret) }

func (s *JWTService) validate(tokenStr, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) { return nil, ErrTokenExpired }
		return nil, ErrTokenInvalid
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid { return nil, ErrTokenInvalid }
	return claims, nil
}

var (
	ErrTokenExpired = errors.New("token has expired")
	ErrTokenInvalid = errors.New("token is invalid")
)
