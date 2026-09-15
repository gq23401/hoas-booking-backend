package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	appauth "github.com/yourusername/hoas-booking/internal/application/auth"
	"github.com/yourusername/hoas-booking/internal/application/ports"
	infraauth "github.com/yourusername/hoas-booking/internal/infrastructure/auth"
	domainuser "github.com/yourusername/hoas-booking/internal/domain/user"
)

type AuthHandler struct {
	register   *appauth.RegisterUseCase
	login      *appauth.LoginUseCase
	jwtService *infraauth.JWTService
	users      ports.UserRepository
}

func NewAuthHandler(register *appauth.RegisterUseCase, login *appauth.LoginUseCase, jwtService *infraauth.JWTService, users ports.UserRepository) *AuthHandler {
	return &AuthHandler{register: register, login: login, jwtService: jwtService, users: users}
}

func (h *AuthHandler) RegisterRoutes(e *echo.Echo) {
	g := e.Group("/api/v1/auth")
	g.POST("/register", h.Register)
	g.POST("/login", h.Login)
	g.POST("/refresh", h.Refresh)
	g.POST("/logout", h.Logout)
}

type AuthResponse struct {
	AccessToken string      `json:"access_token"`
	User        UserPayload `json:"user"`
}

type UserPayload struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
	RoomCount int    `json:"room_count,omitempty"`
}

func toUserPayload(u *domainuser.User) UserPayload {
	p := UserPayload{ID: u.ID.String(), Email: u.Email, FirstName: u.FirstName, LastName: u.LastName, Role: string(u.Role)}
	if u.Apartment != nil { p.RoomCount = u.Apartment.RoomCount }
	return p
}

func (h *AuthHandler) Register(c echo.Context) error {
	type requestBody struct {
		Email     string `json:"email"      validate:"required,email"`
		Password  string `json:"password"   validate:"required,min=8,max=72"`
		FirstName string `json:"first_name" validate:"required,max=100"`
		LastName  string `json:"last_name"  validate:"required,max=100"`
		Phone     string `json:"phone"      validate:"omitempty,max=50"`
		LeaseCode string `json:"lease_code" validate:"required"`
	}
	var body requestBody
	if err := c.Bind(&body); err != nil { return echo.NewHTTPError(http.StatusBadRequest, "invalid request body") }
	if err := c.Validate(&body); err != nil { return echo.NewHTTPError(http.StatusBadRequest, err.Error()) }

	out, err := h.register.Execute(c.Request().Context(), appauth.RegisterInput{Email: body.Email, Password: body.Password, FirstName: body.FirstName, LastName: body.LastName, Phone: body.Phone, LeaseCode: body.LeaseCode})
	if err != nil { return h.mapAuthError(err) }
	return h.respondWithTokens(c, out.User, http.StatusCreated)
}

func (h *AuthHandler) Login(c echo.Context) error {
	type requestBody struct {
		Email    string `json:"email"    validate:"required,email"`
		Password string `json:"password" validate:"required"`
	}
	var body requestBody
	if err := c.Bind(&body); err != nil { return echo.NewHTTPError(http.StatusBadRequest, "invalid request body") }
	if err := c.Validate(&body); err != nil { return echo.NewHTTPError(http.StatusBadRequest, err.Error()) }

	out, err := h.login.Execute(c.Request().Context(), appauth.LoginInput{Email: body.Email, Password: body.Password})
	if err != nil { return h.mapAuthError(err) }
	return h.respondWithTokens(c, out.User, http.StatusOK)
}

func (h *AuthHandler) Refresh(c echo.Context) error {
	cookie, err := c.Cookie("refresh_token")
	if err != nil { return echo.NewHTTPError(http.StatusUnauthorized, "no refresh token") }
	claims, err := h.jwtService.ValidateRefreshToken(cookie.Value)
	if err != nil { return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired refresh token") }
	record, err := h.users.FindRefreshToken(c.Request().Context(), cookie.Value)
	if err != nil || record.RevokedAt != nil { return echo.NewHTTPError(http.StatusUnauthorized, "refresh token revoked") }
	if time.Now().After(record.ExpiresAt) { return echo.NewHTTPError(http.StatusUnauthorized, "refresh token expired") }
	accessToken, err := h.jwtService.IssueAccessToken(claims.UserID, claims.OrgID, claims.Role)
	if err != nil { return echo.NewHTTPError(http.StatusInternalServerError, "could not issue token") }
	return c.JSON(http.StatusOK, map[string]string{"access_token": accessToken})
}

func (h *AuthHandler) Logout(c echo.Context) error {
	if cookie, err := c.Cookie("refresh_token"); err == nil {
		_ = h.users.RevokeRefreshToken(c.Request().Context(), cookie.Value)
	}
	h.clearRefreshCookie(c)
	return c.JSON(http.StatusOK, map[string]string{"status": "logged out"})
}

func (h *AuthHandler) respondWithTokens(c echo.Context, u *domainuser.User, status int) error {
	accessToken, err := h.jwtService.IssueAccessToken(u.ID, u.OrgID, u.Role)
	if err != nil { return echo.NewHTTPError(http.StatusInternalServerError, "could not issue access token") }
	refreshToken, err := h.jwtService.IssueRefreshToken(u.ID, u.OrgID, u.Role)
	if err != nil { return echo.NewHTTPError(http.StatusInternalServerError, "could not issue refresh token") }
	_ = h.users.SaveRefreshToken(c.Request().Context(), u.ID, refreshToken, time.Now().UTC().Add(7*24*time.Hour))
	isSecure := c.Request().TLS != nil
	c.SetCookie(&http.Cookie{Name: "refresh_token", Value: refreshToken, Path: "/api/v1/auth", HttpOnly: true, Secure: isSecure, SameSite: http.SameSiteStrictMode, MaxAge: 7 * 24 * 60 * 60})
	return c.JSON(status, AuthResponse{AccessToken: accessToken, User: toUserPayload(u)})
}

func (h *AuthHandler) clearRefreshCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{Name: "refresh_token", Value: "", Path: "/api/v1/auth", HttpOnly: true, MaxAge: -1})
}

func (h *AuthHandler) mapAuthError(err error) error {
	switch {
	case errors.Is(err, appauth.ErrInvalidCredentials): return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	case errors.Is(err, appauth.ErrInvalidLeaseCode): return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	case errors.Is(err, appauth.ErrApartmentOccupied), errors.Is(err, appauth.ErrEmailAlreadyRegistered): return echo.NewHTTPError(http.StatusConflict, err.Error())
	default: return echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")
	}
}
