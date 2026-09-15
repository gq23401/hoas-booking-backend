package email

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	domainbooking "github.com/yourusername/hoas-booking/internal/domain/booking"
)

type MockNotifier struct { logger *zap.Logger }

func NewMockNotifier(logger *zap.Logger) *MockNotifier { return &MockNotifier{logger: logger} }

func (n *MockNotifier) SendBookingConfirmation(ctx context.Context, userID uuid.UUID, b *domainbooking.LaundryBooking) error {
	if b == nil {
		n.logger.Info("mock email: sauna booking confirmation", zap.String("user_id", userID.String()))
		return nil
	}
	n.logger.Info("mock email: laundry booking confirmation", zap.String("user_id", userID.String()), zap.String("booking_id", b.ID.String()), zap.Time("starts_at", b.StartsAt))
	return nil
}

func (n *MockNotifier) NotifyWaitlistSlotAvailable(ctx context.Context, facilityID uuid.UUID, slotStartsAt time.Time, waitlistUsers []uuid.UUID) error {
	n.logger.Info("mock email: slot available", zap.String("facility_id", facilityID.String()), zap.Int("users", len(waitlistUsers)))
	return nil
}
