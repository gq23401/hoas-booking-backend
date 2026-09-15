package booking

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	domainbooking "github.com/yourusername/hoas-booking/internal/domain/booking"
	"github.com/yourusername/hoas-booking/internal/application/ports"
)

type CreateLaundryBookingInput struct {
	UserID uuid.UUID; OrgID uuid.UUID; MachineID uuid.UUID; StartsAt time.Time
}
type CreateLaundryBookingOutput struct { Booking *domainbooking.LaundryBooking }

type Notifier interface {
	SendBookingConfirmation(ctx context.Context, userID uuid.UUID, b *domainbooking.LaundryBooking) error
}

type CreateLaundryBookingUseCase struct {
	bookings ports.BookingRepository; machines ports.MachineRepository
	quotas   ports.QuotaRepository;   users    ports.UserRepository
	waitlist ports.WaitlistRepository; notifier Notifier
}

func NewCreateLaundryBookingUseCase(bookings ports.BookingRepository, machines ports.MachineRepository, quotas ports.QuotaRepository, users ports.UserRepository, waitlist ports.WaitlistRepository, notifier Notifier) *CreateLaundryBookingUseCase {
	return &CreateLaundryBookingUseCase{bookings: bookings, machines: machines, quotas: quotas, users: users, waitlist: waitlist, notifier: notifier}
}

func (uc *CreateLaundryBookingUseCase) Execute(ctx context.Context, in CreateLaundryBookingInput) (*CreateLaundryBookingOutput, error) {
	if err := domainbooking.ValidateLaundrySlot(in.StartsAt); err != nil { return nil, err }
	endsAt := in.StartsAt.Add(domainbooking.SlotDuration)

	machine, err := uc.machines.FindByID(ctx, in.MachineID)
	if err != nil { return nil, fmt.Errorf("machine lookup: %w", err) }

	hasAccess, err := uc.machines.HasAccess(ctx, in.UserID, machine.LaundryRoomID)
	if err != nil { return nil, fmt.Errorf("access check: %w", err) }
	if !hasAccess { return nil, domainbooking.ErrAccessDenied }
	if !machine.IsBookable() { return nil, domainbooking.ErrMachineNotAvailable }

	maintenanceWindows, err := uc.machines.GetActiveMaintenanceWindows(ctx, in.MachineID, in.StartsAt, endsAt)
	if err != nil { return nil, fmt.Errorf("maintenance check: %w", err) }
	if len(maintenanceWindows) > 0 { return nil, domainbooking.ErrMachineUnderMaint }

	u, err := uc.users.FindByID(ctx, in.UserID)
	if err != nil { return nil, fmt.Errorf("user lookup: %w", err) }

	apartment, err := uc.users.FindApartmentByUserID(ctx, in.UserID)
	if err != nil { return nil, domainbooking.ErrAccessDenied }
	u.Apartment = apartment

	year, month := in.StartsAt.UTC().Year(), int(in.StartsAt.UTC().Month())
	quota, err := uc.quotas.GetOrCreateMonthlyQuota(ctx, in.UserID, in.OrgID, year, month, u.MonthlyLaundrySlots())
	if err != nil { return nil, fmt.Errorf("quota lookup: %w", err) }
	if quota.IsExhausted() { return nil, domainbooking.ErrQuotaExceeded }

	slots, err := uc.bookings.GetMachineDayAvailability(ctx, in.MachineID, in.StartsAt)
	if err != nil { return nil, fmt.Errorf("availability check: %w", err) }
	for _, s := range slots {
		if s.StartsAt.Equal(in.StartsAt) && !s.IsAvailable { return nil, domainbooking.ErrMachineNotAvailable }
	}

	b := &domainbooking.LaundryBooking{
		ID: uuid.New(), OrgID: in.OrgID, UserID: in.UserID, MachineID: in.MachineID,
		StartsAt: in.StartsAt, EndsAt: endsAt, Status: domainbooking.StatusConfirmed,
		QuotaConsumed: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := uc.bookings.CreateLaundryBooking(ctx, b); err != nil {
		return nil, fmt.Errorf("create booking: %w", err)
	}
	_ = uc.quotas.IncrementMonthlyQuota(ctx, in.UserID, year, month)

	go func() {
		bgCtx := context.Background()
		_ = uc.notifier.SendBookingConfirmation(bgCtx, in.UserID, b)
	}()

	return &CreateLaundryBookingOutput{Booking: b}, nil
}
