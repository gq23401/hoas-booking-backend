package booking

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	domainbooking "github.com/yourusername/hoas-booking/internal/domain/booking"
	"github.com/yourusername/hoas-booking/internal/application/ports"
)

type CreateSaunaBookingInput struct {
	UserID uuid.UUID; OrgID uuid.UUID; SaunaID uuid.UUID; StartsAt time.Time
}
type CreateSaunaBookingOutput struct { Booking *domainbooking.SaunaBooking }

type CreateSaunaBookingUseCase struct {
	bookings ports.BookingRepository; quotas ports.QuotaRepository; notifier Notifier
}

func NewCreateSaunaBookingUseCase(bookings ports.BookingRepository, quotas ports.QuotaRepository, notifier Notifier) *CreateSaunaBookingUseCase {
	return &CreateSaunaBookingUseCase{bookings: bookings, quotas: quotas, notifier: notifier}
}

func (uc *CreateSaunaBookingUseCase) Execute(ctx context.Context, in CreateSaunaBookingInput) (*CreateSaunaBookingOutput, error) {
	if err := domainbooking.ValidateSaunaSlot(in.StartsAt); err != nil { return nil, err }
	endsAt := in.StartsAt.Add(domainbooking.SlotDuration)

	weekStart := domainbooking.MondayOf(in.StartsAt)
	quota, err := uc.quotas.GetOrCreateWeeklySaunaQuota(ctx, in.UserID, in.OrgID, weekStart)
	if err != nil { return nil, fmt.Errorf("quota lookup: %w", err) }
	if quota.IsExhausted() { return nil, domainbooking.ErrSaunaQuotaExceeded }

	slots, err := uc.bookings.GetSaunaDayAvailability(ctx, in.SaunaID, in.StartsAt)
	if err != nil { return nil, fmt.Errorf("availability check: %w", err) }
	for _, s := range slots {
		if s.StartsAt.Equal(in.StartsAt) && !s.IsAvailable { return nil, domainbooking.ErrSaunaNotAvailable }
	}

	b := &domainbooking.SaunaBooking{
		ID: uuid.New(), OrgID: in.OrgID, UserID: in.UserID, SaunaID: in.SaunaID,
		StartsAt: in.StartsAt, EndsAt: endsAt, Status: domainbooking.StatusConfirmed,
		QuotaConsumed: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := uc.bookings.CreateSaunaBooking(ctx, b); err != nil {
		return nil, fmt.Errorf("create sauna booking: %w", err)
	}
	_ = uc.quotas.IncrementWeeklySaunaQuota(ctx, in.UserID, weekStart)

	go func() {
		_ = uc.notifier.SendBookingConfirmation(context.Background(), in.UserID, nil)
	}()

	return &CreateSaunaBookingOutput{Booking: b}, nil
}
