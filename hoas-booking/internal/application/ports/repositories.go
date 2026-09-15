package ports

import (
	"context"
	"time"
	"github.com/google/uuid"
	"github.com/yourusername/hoas-booking/internal/domain/booking"
	"github.com/yourusername/hoas-booking/internal/domain/machine"
	"github.com/yourusername/hoas-booking/internal/domain/user"
)

type UserRepository interface {
	Create(ctx context.Context, u *user.User, passwordHash string) error
	FindByID(ctx context.Context, id uuid.UUID) (*user.User, error)
	FindByEmail(ctx context.Context, email string) (*user.User, error)
	FindByOrgID(ctx context.Context, orgID uuid.UUID, filters UserFilters) ([]*user.User, int, error)
	Update(ctx context.Context, u *user.User) error
	Deactivate(ctx context.Context, id uuid.UUID) error
	GetPasswordHash(ctx context.Context, userID uuid.UUID) (string, error)
	CreateApartment(ctx context.Context, a *user.Apartment) error
	FindApartmentByLeaseCode(ctx context.Context, leaseCode string) (*user.Apartment, error)
	FindApartmentByUserID(ctx context.Context, userID uuid.UUID) (*user.Apartment, error)
	UpdateApartmentRoomCount(ctx context.Context, apartmentID uuid.UUID, roomCount int) error
	AssignUserToApartment(ctx context.Context, userID, apartmentID uuid.UUID) error
	SetBankIDVerified(ctx context.Context, userID uuid.UUID, verifiedAt time.Time) error
	SaveRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error
	FindRefreshToken(ctx context.Context, tokenHash string) (*RefreshTokenRecord, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
	RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error
}

type UserFilters struct {
	Role string; IsActive *bool; Search string; Limit int; Offset int
}

type RefreshTokenRecord struct {
	UserID uuid.UUID; ExpiresAt time.Time; RevokedAt *time.Time
}

type MachineRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*machine.Machine, error)
	FindByRoomID(ctx context.Context, roomID uuid.UUID) ([]*machine.Machine, error)
	FindRoomByID(ctx context.Context, roomID uuid.UUID) (*machine.LaundryRoom, error)
	FindRoomsByBuildingID(ctx context.Context, buildingID uuid.UUID) ([]*machine.LaundryRoom, error)
	GrantAccess(ctx context.Context, userID, laundryRoomID, grantedBy uuid.UUID) error
	RevokeAccess(ctx context.Context, userID, laundryRoomID uuid.UUID) error
	HasAccess(ctx context.Context, userID, laundryRoomID uuid.UUID) (bool, error)
	GetAccessibleRooms(ctx context.Context, userID uuid.UUID) ([]*machine.LaundryRoom, error)
	CreateMaintenanceWindow(ctx context.Context, w *machine.MaintenanceWindow) error
	GetActiveMaintenanceWindows(ctx context.Context, machineID uuid.UUID, from, to time.Time) ([]*machine.MaintenanceWindow, error)
	UpdateMachineStatus(ctx context.Context, machineID uuid.UUID, status machine.Status) error
}

type BookingRepository interface {
	CreateLaundryBooking(ctx context.Context, b *booking.LaundryBooking) error
	FindLaundryBookingByID(ctx context.Context, id uuid.UUID) (*booking.LaundryBooking, error)
	UpdateLaundryBookingStatus(ctx context.Context, id uuid.UUID, status booking.Status, cancelledAt *time.Time, reason string) error
	GetUserLaundryBookings(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]*booking.LaundryBooking, error)
	GetUpcomingUserLaundryBookings(ctx context.Context, userID uuid.UUID) ([]*booking.LaundryBooking, error)
	GetMachineDayAvailability(ctx context.Context, machineID uuid.UUID, date time.Time) ([]TimeSlot, error)
	CreateSaunaBooking(ctx context.Context, b *booking.SaunaBooking) error
	FindSaunaBookingByID(ctx context.Context, id uuid.UUID) (*booking.SaunaBooking, error)
	UpdateSaunaBookingStatus(ctx context.Context, id uuid.UUID, status booking.Status, cancelledAt *time.Time) error
	GetUserSaunaBookings(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]*booking.SaunaBooking, error)
	GetSaunaDayAvailability(ctx context.Context, saunaID uuid.UUID, date time.Time) ([]TimeSlot, error)
	FindNoShows(ctx context.Context, cutoff time.Time) ([]*booking.LaundryBooking, error)
	FindSaunaNoShows(ctx context.Context, cutoff time.Time) ([]*booking.SaunaBooking, error)
}

type TimeSlot struct {
	StartsAt time.Time; IsAvailable bool; MachineID uuid.UUID
}

type QuotaRepository interface {
	GetOrCreateMonthlyQuota(ctx context.Context, userID, orgID uuid.UUID, year, month, slotsTotal int) (*MonthlyQuota, error)
	IncrementMonthlyQuota(ctx context.Context, userID uuid.UUID, year, month int) error
	ResetMonthlyQuotas(ctx context.Context, orgID uuid.UUID, year, month int) error
	GetOrCreateWeeklySaunaQuota(ctx context.Context, userID, orgID uuid.UUID, weekStart time.Time) (*WeeklySaunaQuota, error)
	IncrementWeeklySaunaQuota(ctx context.Context, userID uuid.UUID, weekStart time.Time) error
}

type MonthlyQuota struct { SlotsTotal int; SlotsUsed int }
func (q *MonthlyQuota) Remaining() int { return q.SlotsTotal - q.SlotsUsed }
func (q *MonthlyQuota) IsExhausted() bool { return q.SlotsUsed >= q.SlotsTotal }

type WeeklySaunaQuota struct { SlotsUsed int }
func (q *WeeklySaunaQuota) IsExhausted() bool { return q.SlotsUsed >= 1 }

type WaitlistRepository interface {
	Add(ctx context.Context, entry *WaitlistEntry) error
	Remove(ctx context.Context, userID, facilityID uuid.UUID, slotStartsAt time.Time) error
	GetForSlot(ctx context.Context, facilityID uuid.UUID, slotStartsAt time.Time) ([]*WaitlistEntry, error)
	MarkNotified(ctx context.Context, facilityID uuid.UUID, slotStartsAt time.Time) error
	MarkClaimed(ctx context.Context, userID, facilityID uuid.UUID, slotStartsAt time.Time) error
}

type WaitlistEntry struct {
	ID uuid.UUID; OrgID uuid.UUID; UserID uuid.UUID; FacilityType string; FacilityID uuid.UUID
	SlotStartsAt time.Time; NotifiedAt *time.Time; CreatedAt time.Time
}

type NotificationRepository interface {
	Log(ctx context.Context, entry *NotificationLogEntry) error
}

type NotificationLogEntry struct {
	OrgID uuid.UUID; UserID uuid.UUID; Channel string; Type string
	Subject string; Body string; Metadata map[string]any
}
