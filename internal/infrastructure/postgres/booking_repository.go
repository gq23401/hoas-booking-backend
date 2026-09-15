package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
	_ "time/tzdata"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	domainbooking "github.com/yourusername/hoas-booking/internal/domain/booking"
	"github.com/yourusername/hoas-booking/internal/application/ports"
)

type BookingRepository struct { db *sqlx.DB }

func NewBookingRepository(db *sqlx.DB) *BookingRepository { return &BookingRepository{db: db} }

type laundryBookingRow struct {
	ID uuid.UUID `db:"id"`; OrgID uuid.UUID `db:"org_id"`; UserID uuid.UUID `db:"user_id"`; MachineID uuid.UUID `db:"machine_id"`
	StartsAt time.Time `db:"starts_at"`; EndsAt time.Time `db:"ends_at"`; Status string `db:"status"`; QuotaConsumed bool `db:"quota_consumed"`
	CancelledAt sql.NullTime `db:"cancelled_at"`; CancelledReason sql.NullString `db:"cancelled_reason"`; CreatedAt time.Time `db:"created_at"`; UpdatedAt time.Time `db:"updated_at"`
}

func (r *laundryBookingRow) toDomain() *domainbooking.LaundryBooking {
	b := &domainbooking.LaundryBooking{ID: r.ID, OrgID: r.OrgID, UserID: r.UserID, MachineID: r.MachineID, StartsAt: r.StartsAt, EndsAt: r.EndsAt, Status: domainbooking.Status(r.Status), QuotaConsumed: r.QuotaConsumed, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
	if r.CancelledAt.Valid { t := r.CancelledAt.Time; b.CancelledAt = &t }
	if r.CancelledReason.Valid { b.CancelledReason = r.CancelledReason.String }
	return b
}

func (r *BookingRepository) CreateLaundryBooking(ctx context.Context, b *domainbooking.LaundryBooking) error {
	const q = `INSERT INTO laundry_bookings (id, org_id, user_id, machine_id, starts_at, ends_at, status, quota_consumed, created_at, updated_at) VALUES (:id, :org_id, :user_id, :machine_id, :starts_at, :ends_at, :status, :quota_consumed, :created_at, :updated_at)`
	row := laundryBookingRow{ID: b.ID, OrgID: b.OrgID, UserID: b.UserID, MachineID: b.MachineID, StartsAt: b.StartsAt, EndsAt: b.EndsAt, Status: string(b.Status), QuotaConsumed: b.QuotaConsumed, CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt}
	_, err := r.db.NamedExecContext(ctx, q, row)
	if err != nil { return fmt.Errorf("insert laundry booking: %w", err) }
	return nil
}

func (r *BookingRepository) FindLaundryBookingByID(ctx context.Context, id uuid.UUID) (*domainbooking.LaundryBooking, error) {
	var row laundryBookingRow
	if err := r.db.GetContext(ctx, &row, `SELECT * FROM laundry_bookings WHERE id = $1`, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) { return nil, domainbooking.ErrBookingNotFound }
		return nil, fmt.Errorf("find laundry booking: %w", err)
	}
	return row.toDomain(), nil
}

func (r *BookingRepository) UpdateLaundryBookingStatus(ctx context.Context, id uuid.UUID, status domainbooking.Status, cancelledAt *time.Time, reason string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE laundry_bookings SET status = $1, cancelled_at = $2, cancelled_reason = $3, updated_at = NOW() WHERE id = $4`, string(status), cancelledAt, reason, id)
	return err
}

func (r *BookingRepository) GetUserLaundryBookings(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]*domainbooking.LaundryBooking, error) {
	var rows []laundryBookingRow
	if err := r.db.SelectContext(ctx, &rows, `SELECT * FROM laundry_bookings WHERE user_id = $1 AND starts_at >= $2 AND starts_at < $3 ORDER BY starts_at ASC`, userID, from, to); err != nil {
		return nil, err
	}
	out := make([]*domainbooking.LaundryBooking, len(rows))
	for i, row := range rows { row := row; out[i] = row.toDomain() }
	return out, nil
}

func (r *BookingRepository) GetUpcomingUserLaundryBookings(ctx context.Context, userID uuid.UUID) ([]*domainbooking.LaundryBooking, error) {
	var rows []laundryBookingRow
	if err := r.db.SelectContext(ctx, &rows, `SELECT * FROM laundry_bookings WHERE user_id = $1 AND status = 'confirmed' AND starts_at > NOW() ORDER BY starts_at ASC LIMIT 50`, userID); err != nil {
		return nil, err
	}
	out := make([]*domainbooking.LaundryBooking, len(rows))
	for i, row := range rows { row := row; out[i] = row.toDomain() }
	return out, nil
}

func (r *BookingRepository) GetMachineDayAvailability(ctx context.Context, machineID uuid.UUID, date time.Time) ([]ports.TimeSlot, error) {
	loc, err := time.LoadLocation("Europe/Helsinki")
	if err != nil { loc = time.FixedZone("EET", 3*60*60) }
	year, month, day := date.Date()
	openTime  := time.Date(year, month, day, domainbooking.LaundryOpenHour,  0, 0, 0, loc).UTC()
	closeTime := time.Date(year, month, day, domainbooking.LaundryCloseHour, 0, 0, 0, loc).UTC()

	const q = `WITH slots AS (SELECT generate_series($1::timestamptz, $2::timestamptz - interval '1 hour', interval '1 hour') AS slot_start), booked AS (SELECT starts_at FROM laundry_bookings WHERE machine_id = $3 AND status = 'confirmed' AND starts_at >= $1 AND starts_at < $2) SELECT s.slot_start AS starts_at, (b.starts_at IS NULL) AS is_available FROM slots s LEFT JOIN booked b ON b.starts_at = s.slot_start ORDER BY s.slot_start`
	type row struct { StartsAt time.Time `db:"starts_at"`; IsAvailable bool `db:"is_available"` }
	var rows []row
	if err := r.db.SelectContext(ctx, &rows, q, openTime, closeTime, machineID); err != nil { return nil, fmt.Errorf("get availability: %w", err) }
	slots := make([]ports.TimeSlot, len(rows))
	for i, row := range rows { slots[i] = ports.TimeSlot{StartsAt: row.StartsAt, IsAvailable: row.IsAvailable, MachineID: machineID} }
	return slots, nil
}

func (r *BookingRepository) FindNoShows(ctx context.Context, cutoff time.Time) ([]*domainbooking.LaundryBooking, error) {
	var rows []laundryBookingRow
	if err := r.db.SelectContext(ctx, &rows, `SELECT * FROM laundry_bookings WHERE status = 'confirmed' AND starts_at < $1`, cutoff); err != nil { return nil, err }
	out := make([]*domainbooking.LaundryBooking, len(rows))
	for i, row := range rows { row := row; out[i] = row.toDomain() }
	return out, nil
}

type saunaBookingRow struct {
	ID uuid.UUID `db:"id"`; OrgID uuid.UUID `db:"org_id"`; UserID uuid.UUID `db:"user_id"`; SaunaID uuid.UUID `db:"sauna_id"`
	StartsAt time.Time `db:"starts_at"`; EndsAt time.Time `db:"ends_at"`; Status string `db:"status"`; QuotaConsumed bool `db:"quota_consumed"`
	CancelledAt sql.NullTime `db:"cancelled_at"`; CreatedAt time.Time `db:"created_at"`; UpdatedAt time.Time `db:"updated_at"`
}

func (r *saunaBookingRow) toDomain() *domainbooking.SaunaBooking {
	b := &domainbooking.SaunaBooking{ID: r.ID, OrgID: r.OrgID, UserID: r.UserID, SaunaID: r.SaunaID, StartsAt: r.StartsAt, EndsAt: r.EndsAt, Status: domainbooking.Status(r.Status), QuotaConsumed: r.QuotaConsumed, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
	if r.CancelledAt.Valid { t := r.CancelledAt.Time; b.CancelledAt = &t }
	return b
}

func (r *BookingRepository) CreateSaunaBooking(ctx context.Context, b *domainbooking.SaunaBooking) error {
	const q = `INSERT INTO sauna_bookings (id, org_id, user_id, sauna_id, starts_at, ends_at, status, quota_consumed, created_at, updated_at) VALUES (:id, :org_id, :user_id, :sauna_id, :starts_at, :ends_at, :status, :quota_consumed, :created_at, :updated_at)`
	row := saunaBookingRow{ID: b.ID, OrgID: b.OrgID, UserID: b.UserID, SaunaID: b.SaunaID, StartsAt: b.StartsAt, EndsAt: b.EndsAt, Status: string(b.Status), QuotaConsumed: b.QuotaConsumed, CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt}
	_, err := r.db.NamedExecContext(ctx, q, row)
	return err
}

func (r *BookingRepository) FindSaunaBookingByID(ctx context.Context, id uuid.UUID) (*domainbooking.SaunaBooking, error) {
	var row saunaBookingRow
	if err := r.db.GetContext(ctx, &row, `SELECT * FROM sauna_bookings WHERE id = $1`, id); err != nil { return nil, err }
	return row.toDomain(), nil
}

func (r *BookingRepository) UpdateSaunaBookingStatus(ctx context.Context, id uuid.UUID, status domainbooking.Status, cancelledAt *time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE sauna_bookings SET status=$1, cancelled_at=$2, updated_at=NOW() WHERE id=$3`, string(status), cancelledAt, id)
	return err
}

func (r *BookingRepository) GetUserSaunaBookings(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]*domainbooking.SaunaBooking, error) {
	var rows []saunaBookingRow
	if err := r.db.SelectContext(ctx, &rows, `SELECT * FROM sauna_bookings WHERE user_id=$1 AND starts_at>=$2 AND starts_at<$3 ORDER BY starts_at ASC`, userID, from, to); err != nil { return nil, err }
	out := make([]*domainbooking.SaunaBooking, len(rows))
	for i, row := range rows { row := row; out[i] = row.toDomain() }
	return out, nil
}

func (r *BookingRepository) GetSaunaDayAvailability(ctx context.Context, saunaID uuid.UUID, date time.Time) ([]ports.TimeSlot, error) {
	loc, err := time.LoadLocation("Europe/Helsinki")
	if err != nil { loc = time.FixedZone("EET", 3*60*60) }
	year, month, day := date.Date()
	openTime  := time.Date(year, month, day, domainbooking.SaunaOpenHour,  0, 0, 0, loc).UTC()
	closeTime := time.Date(year, month, day, domainbooking.SaunaCloseHour, 0, 0, 0, loc).UTC()
	const q = `WITH slots AS (SELECT generate_series($1::timestamptz, $2::timestamptz - interval '1 hour', interval '1 hour') AS slot_start), booked AS (SELECT starts_at FROM sauna_bookings WHERE sauna_id=$3 AND status='confirmed' AND starts_at>=$1 AND starts_at<$2) SELECT s.slot_start AS starts_at, (b.starts_at IS NULL) AS is_available FROM slots s LEFT JOIN booked b ON b.starts_at=s.slot_start ORDER BY s.slot_start`
	type row struct { StartsAt time.Time `db:"starts_at"`; IsAvailable bool `db:"is_available"` }
	var rows []row
	if err := r.db.SelectContext(ctx, &rows, q, openTime, closeTime, saunaID); err != nil { return nil, err }
	slots := make([]ports.TimeSlot, len(rows))
	for i, row := range rows { slots[i] = ports.TimeSlot{StartsAt: row.StartsAt, IsAvailable: row.IsAvailable} }
	return slots, nil
}

func (r *BookingRepository) FindSaunaNoShows(ctx context.Context, cutoff time.Time) ([]*domainbooking.SaunaBooking, error) {
	var rows []saunaBookingRow
	if err := r.db.SelectContext(ctx, &rows, `SELECT * FROM sauna_bookings WHERE status='confirmed' AND starts_at<$1`, cutoff); err != nil { return nil, err }
	out := make([]*domainbooking.SaunaBooking, len(rows))
	for i, row := range rows { row := row; out[i] = row.toDomain() }
	return out, nil
}
