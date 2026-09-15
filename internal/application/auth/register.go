package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"github.com/yourusername/hoas-booking/internal/application/ports"
	domainuser "github.com/yourusername/hoas-booking/internal/domain/user"
)

type RegisterInput struct {
	Email string; Password string; FirstName string; LastName string; Phone string; LeaseCode string
}

type RegisterOutput struct {
	User *domainuser.User; Apartment *domainuser.Apartment
}

type RegisterUseCase struct { users ports.UserRepository }

func NewRegisterUseCase(users ports.UserRepository) *RegisterUseCase {
	return &RegisterUseCase{users: users}
}

func (uc *RegisterUseCase) Execute(ctx context.Context, in RegisterInput) (*RegisterOutput, error) {
	apartment, err := uc.users.FindApartmentByLeaseCode(ctx, in.LeaseCode)
	if err != nil {
		return nil, ErrInvalidLeaseCode
	}
	if apartment.UserID != (uuid.UUID{}) {
		return nil, ErrApartmentOccupied
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), 12)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	u := &domainuser.User{
		ID: uuid.New(), OrgID: apartment.OrgID, Email: in.Email,
		FirstName: in.FirstName, LastName: in.LastName, Phone: in.Phone,
		Role: domainuser.RoleTenant, IsActive: true,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := uc.users.Create(ctx, u, string(hash)); err != nil {
		if strings.Contains(err.Error(), "23505") {
			return nil, ErrEmailAlreadyRegistered
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	if err := uc.users.AssignUserToApartment(ctx, u.ID, apartment.ID); err != nil {
		return nil, fmt.Errorf("assign apartment: %w", err)
	}
	apartment.UserID = u.ID
	u.Apartment = apartment
	return &RegisterOutput{User: u, Apartment: apartment}, nil
}

var (
	ErrInvalidLeaseCode       = errors.New("lease code not found — check with your building manager")
	ErrApartmentOccupied      = errors.New("this apartment already has a registered tenant")
	ErrEmailAlreadyRegistered = errors.New("an account with this email already exists")
)
