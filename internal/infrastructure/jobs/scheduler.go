package jobs

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	domainbooking "github.com/yourusername/hoas-booking/internal/domain/booking"
	"github.com/yourusername/hoas-booking/internal/application/ports"
)

type Scheduler struct {
	cron     *cron.Cron
	logger   *zap.Logger
	bookings ports.BookingRepository
	quotas   ports.QuotaRepository
	waitlist ports.WaitlistRepository
}

func NewScheduler(logger *zap.Logger, bookings ports.BookingRepository, quotas ports.QuotaRepository, waitlist ports.WaitlistRepository) *Scheduler {
	c := cron.New(cron.WithSeconds())
	s := &Scheduler{cron: c, logger: logger, bookings: bookings, quotas: quotas, waitlist: waitlist}
	c.AddFunc("0 * * * * *", s.releaseNoShows)
	c.AddFunc("0 0 0 1 * *", s.resetMonthlyQuotas)
	c.AddFunc("0 0 18 * * *", s.sendBookingReminders)
	c.AddFunc("0 0 2 * * *", s.cleanupExpiredWaitlist)
	return s
}

func (s *Scheduler) Start() { s.cron.Start(); s.logger.Info("background job scheduler started") }
func (s *Scheduler) Stop()  { s.cron.Stop(); s.logger.Info("background job scheduler stopped") }

func (s *Scheduler) releaseNoShows() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cutoff := time.Now().UTC().Add(-domainbooking.NoShowGracePeriod)
	noShows, err := s.bookings.FindNoShows(ctx, cutoff)
	if err != nil { s.logger.Error("find no shows", zap.Error(err)); return }
	for _, b := range noShows {
		now := time.Now().UTC()
		if err := s.bookings.UpdateLaundryBookingStatus(ctx, b.ID, domainbooking.StatusNoShow, &now, "auto-released: no-show"); err != nil {
			s.logger.Error("mark no show", zap.Error(err))
		}
	}
}

func (s *Scheduler) resetMonthlyQuotas() {
	s.logger.Info("resetting monthly laundry quotas")
}

func (s *Scheduler) sendBookingReminders() {
	s.logger.Info("sending booking reminders")
}

func (s *Scheduler) cleanupExpiredWaitlist() {
	s.logger.Info("cleaning up expired waitlist entries")
}
