package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/hoas-booking/config"
)

type BankIDProvider interface {
	InitiateAuth(ctx context.Context, req BankIDAuthRequest) (*BankIDSession, error)
	VerifyCallback(ctx context.Context, sessionToken string) (*BankIDResult, error)
}

type BankIDAuthRequest struct { UserID uuid.UUID; RedirectURL string }
type BankIDSession struct { SessionToken string; RedirectURL string; ExpiresAt time.Time }
type BankIDResult struct { Verified bool; UserID uuid.UUID; FullName string; IDNumber string; VerifiedAt time.Time }

type MockBankIDProvider struct { cfg config.BankIDConfig }

func NewMockBankIDProvider(cfg config.BankIDConfig) BankIDProvider { return &MockBankIDProvider{cfg: cfg} }

func (m *MockBankIDProvider) InitiateAuth(ctx context.Context, req BankIDAuthRequest) (*BankIDSession, error) {
	token, err := generateToken()
	if err != nil { return nil, fmt.Errorf("generate session token: %w", err) }
	return &BankIDSession{
		SessionToken: token,
		RedirectURL:  fmt.Sprintf("/mock-bank-id?session=%s&redirect=%s&delay=%d", token, req.RedirectURL, m.cfg.MockDelay),
		ExpiresAt:    time.Now().Add(10 * time.Minute),
	}, nil
}

func (m *MockBankIDProvider) VerifyCallback(ctx context.Context, sessionToken string) (*BankIDResult, error) {
	return &BankIDResult{Verified: true, FullName: "Mock User", IDNumber: "123456-789A", VerifiedAt: time.Now().UTC()}, nil
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil { return "", err }
	return hex.EncodeToString(b), nil
}
