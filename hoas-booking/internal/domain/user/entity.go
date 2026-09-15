package user

import (
	"time"
	"github.com/google/uuid"
)

type Role string

const (
	RoleSuperAdmin      Role = "super_admin"
	RoleOrgAdmin        Role = "org_admin"
	RoleBuildingManager Role = "building_manager"
	RoleTenant          Role = "tenant"
)

type User struct {
	ID                   uuid.UUID
	OrgID                uuid.UUID
	Email                string
	FirstName            string
	LastName             string
	Phone                string
	Role                 Role
	BankIDVerified       bool
	BankIDLastVerifiedAt *time.Time
	IsActive             bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
	Apartment            *Apartment
}

type Apartment struct {
	ID              uuid.UUID
	OrgID           uuid.UUID
	BuildingID      uuid.UUID
	UserID          uuid.UUID
	ApartmentNumber string
	Floor           int
	RoomCount       int
	LeaseCode       string
	LeaseStartDate  *time.Time
	LeaseEndDate    *time.Time
	IsActive        bool
}

func (u *User) FullName() string { return u.FirstName + " " + u.LastName }
func (u *User) IsAdmin() bool {
	return u.Role == RoleSuperAdmin || u.Role == RoleOrgAdmin || u.Role == RoleBuildingManager
}
func (u *User) MonthlyLaundrySlots() int {
	if u.Apartment == nil { return 0 }
	return u.Apartment.RoomCount * 5
}
func (u *User) CanBookLaundry() bool {
	return u.Role == RoleTenant && u.IsActive && u.Apartment != nil && u.Apartment.IsActive
}
