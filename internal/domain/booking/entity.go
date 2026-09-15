package booking

import (
	"time"
	"github.com/google/uuid"
)

type Status string
const (
	StatusConfirmed Status = "confirmed"
	StatusCancelled Status = "cancelled"
	StatusNoShow    Status = "no_show"
	StatusCompleted Status = "completed"
)

type LaundryBooking struct {
	ID uuid.UUID; OrgID uuid.UUID; UserID uuid.UUID; MachineID uuid.UUID
	StartsAt time.Time; EndsAt time.Time; Status Status
	QuotaConsumed bool; CancelledAt *time.Time; CancelledReason string
	CreatedAt time.Time; UpdatedAt time.Time
}

type SaunaBooking struct {
	ID uuid.UUID; OrgID uuid.UUID; UserID uuid.UUID; SaunaID uuid.UUID
	StartsAt time.Time; EndsAt time.Time; Status Status
	QuotaConsumed bool; CancelledAt *time.Time
	CreatedAt time.Time; UpdatedAt time.Time
}

func (b *LaundryBooking) IsLateCancel() bool { return time.Until(b.StartsAt) < 12*time.Hour }

const (
	SlotDuration           = time.Hour
	LaundryOpenHour        = 7
	LaundryCloseHour       = 22
	SaunaOpenHour          = 16
	SaunaCloseHour         = 21
	NoShowGracePeriod      = 15 * time.Minute
	LaundryMaxAdvanceWeeks = 8
	SaunaMaxAdvanceWeeks   = 2
)

var SaunaEligibleDays = map[time.Weekday]bool{
	time.Thursday: true, time.Friday: true, time.Saturday: true, time.Sunday: true,
}

func ValidateLaundrySlot(t time.Time) error {
	if t.Minute() != 0 || t.Second() != 0 || t.Nanosecond() != 0 { return ErrSlotMustBeOnHour }
	h := t.Hour()
	if h < LaundryOpenHour || h >= LaundryCloseHour { return ErrLaundryOutsideHours }
	if t.Before(time.Now()) { return ErrSlotInPast }
	if t.After(time.Now().AddDate(0, 0, LaundryMaxAdvanceWeeks*7)) { return ErrSlotTooFarAhead }
	return nil
}

func ValidateSaunaSlot(t time.Time) error {
	if t.Minute() != 0 || t.Second() != 0 || t.Nanosecond() != 0 { return ErrSlotMustBeOnHour }
	if !SaunaEligibleDays[t.Weekday()] { return ErrSaunaNotEligibleDay }
	h := t.Hour()
	if h < SaunaOpenHour || h >= SaunaCloseHour { return ErrSaunaOutsideHours }
	if t.Before(time.Now()) { return ErrSlotInPast }
	if t.After(time.Now().AddDate(0, 0, SaunaMaxAdvanceWeeks*7)) { return ErrSlotTooFarAhead }
	return nil
}

func MondayOf(t time.Time) time.Time {
	t = t.UTC().Truncate(24 * time.Hour)
	offset := (int(t.Weekday()) - int(time.Monday) + 7) % 7
	return t.AddDate(0, 0, -offset)
}
