package booking

import "errors"

var (
	ErrSlotMustBeOnHour    = errors.New("booking slot must start on the hour")
	ErrLaundryOutsideHours = errors.New("laundry room is only open 07:00–22:00")
	ErrSaunaOutsideHours   = errors.New("sauna is only open 16:00–21:00")
	ErrSaunaNotEligibleDay = errors.New("sauna is only available Thursday through Sunday")
	ErrSlotInPast          = errors.New("cannot book a slot in the past")
	ErrSlotTooFarAhead     = errors.New("slot is beyond the advance booking window")
	ErrMachineNotAvailable = errors.New("this machine is not available at the requested time")
	ErrSaunaNotAvailable   = errors.New("this sauna slot is already taken")
	ErrQuotaExceeded       = errors.New("monthly laundry quota exceeded")
	ErrSaunaQuotaExceeded  = errors.New("weekly sauna quota exceeded (1 per week)")
	ErrAccessDenied        = errors.New("you do not have access to this laundry room")
	ErrBookingNotFound     = errors.New("booking not found")
	ErrCannotCancelPast    = errors.New("cannot cancel a booking that has already started")
	ErrMachineUnderMaint   = errors.New("machine is under maintenance at this time")
)
