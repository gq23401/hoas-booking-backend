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

type quotaRepository struct { db *sqlx.DB }

func NewQuotaRepository(db *sqlx.DB) ports.QuotaRepository { return &quotaRepository{db: db} }

func (r *quotaRepository) GetOrCreateMonthlyQuota(ctx context.Context, userID, orgID uuid.UUID, year, month, slotsTotal int) (*ports.MonthlyQuota, error) {
	const q = `INSERT INTO monthly_laundry_quota (id, user_id, org_id, year, month, slots_total, slots_used, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, 0, NOW(), NOW()) ON CONFLICT (user_id, year, month) DO UPDATE SET slots_total = EXCLUDED.slots_total, updated_at = NOW() RETURNING slots_total, slots_used`
	var row struct { SlotsTotal int `db:"slots_total"`; SlotsUsed int `db:"slots_used"` }
	if err := r.db.GetContext(ctx, &row, q, uuid.New(), userID, orgID, year, month, slotsTotal); err != nil {
		return nil, fmt.Errorf("get or create monthly quota: %w", err)
	}
	return &ports.MonthlyQuota{SlotsTotal: row.SlotsTotal, SlotsUsed: row.SlotsUsed}, nil
}

func (r *quotaRepository) IncrementMonthlyQuota(ctx context.Context, userID uuid.UUID, year, month int) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil { return fmt.Errorf("begin transaction: %w", err) }
	defer tx.Rollback()
	var row struct { SlotsUsed int `db:"slots_used"`; SlotsTotal int `db:"slots_total"` }
	if err := tx.GetContext(ctx, &row, `SELECT slots_used, slots_total FROM monthly_laundry_quota WHERE user_id = $1 AND year = $2 AND month = $3 FOR UPDATE NOWAIT`, userID, year, month); err != nil {
		if errors.Is(err, sql.ErrNoRows) { return fmt.Errorf("quota row missing") }
		return fmt.Errorf("lock quota row: %w", err)
	}
	if row.SlotsUsed >= row.SlotsTotal { return ErrQuotaExhausted }
	if _, err := tx.ExecContext(ctx, `UPDATE monthly_laundry_quota SET slots_used = slots_used + 1, updated_at = NOW() WHERE user_id = $1 AND year = $2 AND month = $3`, userID, year, month); err != nil {
		return fmt.Errorf("increment quota: %w", err)
	}
	return tx.Commit()
}

func (r *quotaRepository) ResetMonthlyQuotas(ctx context.Context, orgID uuid.UUID, year, month int) error {
	const q = `INSERT INTO monthly_laundry_quota (id, user_id, org_id, year, month, slots_total, slots_used, created_at, updated_at) SELECT gen_random_uuid(), a.user_id, a.org_id, $2, $3, a.room_count * 5, 0, NOW(), NOW() FROM apartments a WHERE a.org_id = $1 AND a.is_active = true AND a.user_id IS NOT NULL ON CONFLICT (user_id, year, month) DO UPDATE SET slots_used = 0, slots_total = EXCLUDED.slots_total, updated_at = NOW()`
	_, err := r.db.ExecContext(ctx, q, orgID, year, month)
	return err
}

func (r *quotaRepository) GetOrCreateWeeklySaunaQuota(ctx context.Context, userID, orgID uuid.UUID, weekStart time.Time) (*ports.WeeklySaunaQuota, error) {
	const q = `INSERT INTO weekly_sauna_quota (id, user_id, org_id, week_start, slots_used, created_at, updated_at) VALUES ($1, $2, $3, $4, 0, NOW(), NOW()) ON CONFLICT (user_id, week_start) DO NOTHING RETURNING slots_used`
	var slotsUsed int
	err := r.db.GetContext(ctx, &slotsUsed, q, uuid.New(), userID, orgID, weekStart.UTC())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if err2 := r.db.GetContext(ctx, &slotsUsed, `SELECT slots_used FROM weekly_sauna_quota WHERE user_id = $1 AND week_start = $2`, userID, weekStart.UTC()); err2 != nil {
				return nil, fmt.Errorf("get weekly sauna quota: %w", err2)
			}
		} else {
			return nil, fmt.Errorf("get or create weekly sauna quota: %w", err)
		}
	}
	return &ports.WeeklySaunaQuota{SlotsUsed: slotsUsed}, nil
}

func (r *quotaRepository) IncrementWeeklySaunaQuota(ctx context.Context, userID uuid.UUID, weekStart time.Time) error {
	result, err := r.db.ExecContext(ctx, `UPDATE weekly_sauna_quota SET slots_used = slots_used + 1, updated_at = NOW() WHERE user_id = $1 AND week_start = $2`, userID, weekStart.UTC())
	if err != nil { return err }
	n, _ := result.RowsAffected()
	if n == 0 { return fmt.Errorf("sauna quota row not found") }
	return nil
}

var ErrQuotaExhausted = errors.New("quota exhausted")
