package auth

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
	"github.com/yourusername/hoas-booking/internal/application/ports"
	domainuser "github.com/yourusername/hoas-booking/internal/domain/user"
)

type LoginInput struct { Email string; Password string }
type LoginOutput struct { User *domainuser.User }

type LoginUseCase struct { users ports.UserRepository }

func NewLoginUseCase(users ports.UserRepository) *LoginUseCase {
	return &LoginUseCase{users: users}
}

func (uc *LoginUseCase) Execute(ctx context.Context, in LoginInput) (*LoginOutput, error) {
	u, err := uc.users.FindByEmail(ctx, in.Email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	hash, err := uc.users.GetPasswordHash(ctx, u.ID)
	if err != nil {
		return nil, fmt.Errorf("get password hash: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("compare hash: %w", err)
	}
	apartment, err := uc.users.FindApartmentByUserID(ctx, u.ID)
	if err == nil {
		u.Apartment = apartment
	}
	return &LoginOutput{User: u}, nil
}

var ErrInvalidCredentials = errors.New("invalid email or password")
