package booking

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	domainbooking "github.com/yourusername/hoas-booking/internal/domain/booking"
	"github.com/yourusername/hoas-booking/internal/application/ports"
)

type CancelLaundryBookingInput struct {
	BookingID uuid.UUID; RequestingUID uuid.UUID; OrgID uuid.UUID; Reason string; AdminOverride bool
}

type WaitlistNotifier interface {
	NotifyWaitlistSlotAvailable(ctx context.Context, facilityID uuid.UUID, slotStartsAt time.Time, waitlistUsers []uuid.UUID) error
}

type CancelLaundryBookingUseCase struct {
	bookings ports.BookingRepository
	waitlist ports.WaitlistRepository
	notifier WaitlistNotifier
}

func NewCancelLaundryBookingUseCase(bookings ports.BookingRepository, waitlist ports.WaitlistRepository, notifier WaitlistNotifier) *CancelLaundryBookingUseCase {
	return &CancelLaundryBookingUseCase{bookings: bookings, waitlist: waitlist, notifier: notifier}
}

func (uc *CancelLaundryBookingUseCase) Execute(ctx context.Context, in CancelLaundryBookingInput) error {
	b, err := uc.bookings.FindLaundryBookingByID(ctx, in.BookingID)
	if err != nil { return domainbooking.ErrBookingNotFound }
	if !in.AdminOverride && b.UserID != in.RequestingUID { return domainbooking.ErrAccessDenied }
	if b.StartsAt.Before(time.Now()) { return domainbooking.ErrCannotCancelPast }

	now := time.Now().UTC()
	if err := uc.bookings.UpdateLaundryBookingStatus(ctx, in.BookingID, domainbooking.StatusCancelled, &now, in.Reason); err != nil {
		return fmt.Errorf("cancel booking: %w", err)
	}

	go func() {
		bgCtx := context.Background()
		waitlisted, err := uc.waitlist.GetForSlot(bgCtx, b.MachineID, b.StartsAt)
		if err != nil || len(waitlisted) == 0 { return }
		userIDs := make([]uuid.UUID, len(waitlisted))
		for i, w := range waitlisted { userIDs[i] = w.UserID }
		_ = uc.notifier.NotifyWaitlistSlotAvailable(bgCtx, b.MachineID, b.StartsAt, userIDs)
		_ = uc.waitlist.MarkNotified(bgCtx, b.MachineID, b.StartsAt)
	}()

	return nil
}
