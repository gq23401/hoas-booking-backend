package middleware

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	infraauth "github.com/yourusername/hoas-booking/internal/infrastructure/auth"
	"github.com/yourusername/hoas-booking/internal/domain/user"
)

const (
	CtxUserID = "user_id"
	CtxOrgID  = "org_id"
	CtxRole   = "role"
)

func JWTAuth(jwtSvc *infraauth.JWTService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			if header == "" { return echo.NewHTTPError(http.StatusUnauthorized, "missing authorization header") }
			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid authorization format")
			}
			claims, err := jwtSvc.ValidateAccessToken(parts[1])
			if err != nil { return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired token") }
			c.Set(CtxUserID, claims.UserID)
			c.Set(CtxOrgID, claims.OrgID)
			c.Set(CtxRole, claims.Role)
			return next(c)
		}
	}
}

func RequireRole(roles ...user.Role) echo.MiddlewareFunc {
	allowed := make(map[user.Role]bool, len(roles))
	for _, r := range roles { allowed[r] = true }
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role, _ := c.Get(CtxRole).(user.Role)
			if !allowed[role] { return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions") }
			return next(c)
		}
	}
}

func GetUserID(c echo.Context) uuid.UUID { id, _ := c.Get(CtxUserID).(uuid.UUID); return id }
func GetOrgID(c echo.Context) uuid.UUID  { id, _ := c.Get(CtxOrgID).(uuid.UUID); return id }
func GetRole(c echo.Context) user.Role   { role, _ := c.Get(CtxRole).(user.Role); return role }
