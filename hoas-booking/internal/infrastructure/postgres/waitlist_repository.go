package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/yourusername/hoas-booking/internal/application/ports"
)

type waitlistRepository struct { db *sqlx.DB }

func NewWaitlistRepository(db *sqlx.DB) ports.WaitlistRepository { return &waitlistRepository{db: db} }

func (r *waitlistRepository) Add(ctx context.Context, entry *ports.WaitlistEntry) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO waitlist (id, org_id, user_id, facility_type, facility_id, slot_starts_at, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (user_id, facility_id, slot_starts_at) DO NOTHING`,
		entry.ID, entry.OrgID, entry.UserID, entry.FacilityType, entry.FacilityID, entry.SlotStartsAt.UTC(), time.Now().UTC())
	return err
}

func (r *waitlistRepository) Remove(ctx context.Context, userID, facilityID uuid.UUID, slotStartsAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM waitlist WHERE user_id = $1 AND facility_id = $2 AND slot_starts_at = $3`, userID, facilityID, slotStartsAt.UTC())
	return err
}

func (r *waitlistRepository) GetForSlot(ctx context.Context, facilityID uuid.UUID, slotStartsAt time.Time) ([]*ports.WaitlistEntry, error) {
	type waitlistRow struct {
		ID uuid.UUID `db:"id"`; OrgID uuid.UUID `db:"org_id"`; UserID uuid.UUID `db:"user_id"`
		FacilityType string `db:"facility_type"`; FacilityID uuid.UUID `db:"facility_id"`
		SlotStartsAt time.Time `db:"slot_starts_at"`; NotifiedAt sql.NullTime `db:"notified_at"`; CreatedAt time.Time `db:"created_at"`
	}
	var rows []waitlistRow
	if err := r.db.SelectContext(ctx, &rows, `SELECT * FROM waitlist WHERE facility_id = $1 AND slot_starts_at = $2 AND notified_at IS NULL AND claimed_at IS NULL AND expired_at IS NULL AND slot_starts_at > NOW() ORDER BY created_at ASC`, facilityID, slotStartsAt.UTC()); err != nil {
		return nil, fmt.Errorf("get waitlist for slot: %w", err)
	}
	entries := make([]*ports.WaitlistEntry, len(rows))
	for i, row := range rows {
		e := &ports.WaitlistEntry{ID: row.ID, OrgID: row.OrgID, UserID: row.UserID, FacilityType: row.FacilityType, FacilityID: row.FacilityID, SlotStartsAt: row.SlotStartsAt, CreatedAt: row.CreatedAt}
		if row.NotifiedAt.Valid { t := row.NotifiedAt.Time; e.NotifiedAt = &t }
		entries[i] = e
	}
	return entries, nil
}

func (r *waitlistRepository) MarkNotified(ctx context.Context, facilityID uuid.UUID, slotStartsAt time.Time) error {
	var notifiedIDs []uuid.UUID
	if err := r.db.SelectContext(ctx, &notifiedIDs, `UPDATE waitlist SET notified_at = NOW() WHERE facility_id = $1 AND slot_starts_at = $2 AND notified_at IS NULL AND claimed_at IS NULL AND expired_at IS NULL RETURNING user_id`, facilityID, slotStartsAt.UTC()); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("mark waitlist notified: %w", err)
	}
	return nil
}

func (r *waitlistRepository) MarkClaimed(ctx context.Context, userID, facilityID uuid.UUID, slotStartsAt time.Time) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE waitlist SET claimed_at = NOW() WHERE user_id = $1 AND facility_id = $2 AND slot_starts_at = $3`, userID, facilityID, slotStartsAt.UTC()); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `UPDATE waitlist SET expired_at = NOW() WHERE facility_id = $2 AND slot_starts_at = $3 AND user_id != $1 AND claimed_at IS NULL`, userID, facilityID, slotStartsAt.UTC())
	return err
}
