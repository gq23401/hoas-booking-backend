package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/yourusername/hoas-booking/internal/application/ports"
	"github.com/yourusername/hoas-booking/internal/domain/user"
)

type userRepository struct { db *sqlx.DB }

func NewUserRepository(db *sqlx.DB) ports.UserRepository { return &userRepository{db: db} }

type userRow struct {
	ID                   uuid.UUID      `db:"id"`
	OrgID                uuid.UUID      `db:"org_id"`
	Email                string         `db:"email"`
	PasswordHash         string         `db:"password_hash"`
	FirstName            string         `db:"first_name"`
	LastName             string         `db:"last_name"`
	Phone                sql.NullString `db:"phone"`
	Role                 string         `db:"role"`
	BankIDVerified       bool           `db:"bank_id_verified"`
	BankIDLastVerifiedAt sql.NullTime   `db:"bank_id_last_verified_at"`
	IsActive             bool           `db:"is_active"`
	CreatedAt            time.Time      `db:"created_at"`
	UpdatedAt            time.Time      `db:"updated_at"`
}

func (r *userRow) toDomain() *user.User {
	u := &user.User{ID: r.ID, OrgID: r.OrgID, Email: r.Email, FirstName: r.FirstName, LastName: r.LastName, Role: user.Role(r.Role), BankIDVerified: r.BankIDVerified, IsActive: r.IsActive, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
	if r.Phone.Valid { u.Phone = r.Phone.String }
	if r.BankIDLastVerifiedAt.Valid { t := r.BankIDLastVerifiedAt.Time; u.BankIDLastVerifiedAt = &t }
	return u
}

type apartmentRow struct {
	ID              uuid.UUID      `db:"id"`
	OrgID           uuid.UUID      `db:"org_id"`
	BuildingID      uuid.UUID      `db:"building_id"`
	UserID          uuid.NullUUID  `db:"user_id"`
	ApartmentNumber string         `db:"apartment_number"`
	Floor           sql.NullInt64  `db:"floor"`
	RoomCount       int            `db:"room_count"`
	LeaseCode       string         `db:"lease_code"`
	LeaseStartDate  sql.NullTime   `db:"lease_start_date"`
	LeaseEndDate    sql.NullTime   `db:"lease_end_date"`
	IsActive        bool           `db:"is_active"`
	CreatedAt       time.Time      `db:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at"`
}

func (r *apartmentRow) toDomain() *user.Apartment {
	a := &user.Apartment{ID: r.ID, OrgID: r.OrgID, BuildingID: r.BuildingID, ApartmentNumber: r.ApartmentNumber, Floor: int(r.Floor.Int64), RoomCount: r.RoomCount, LeaseCode: r.LeaseCode, IsActive: r.IsActive}
	if r.UserID.Valid { a.UserID = r.UserID.UUID }
	if r.LeaseStartDate.Valid { t := r.LeaseStartDate.Time; a.LeaseStartDate = &t }
	if r.LeaseEndDate.Valid { t := r.LeaseEndDate.Time; a.LeaseEndDate = &t }
	return a
}

func (r *userRepository) Create(ctx context.Context, u *user.User, passwordHash string) error {
	const q = `INSERT INTO users (id, org_id, email, password_hash, first_name, last_name, phone, role, bank_id_verified, is_active, created_at, updated_at) VALUES (:id, :org_id, :email, :password_hash, :first_name, :last_name, :phone, :role, :bank_id_verified, :is_active, :created_at, :updated_at)`
	row := struct {
		ID uuid.UUID `db:"id"`; OrgID uuid.UUID `db:"org_id"`; Email string `db:"email"`; PasswordHash string `db:"password_hash"`
		FirstName string `db:"first_name"`; LastName string `db:"last_name"`; Phone sql.NullString `db:"phone"`; Role string `db:"role"`
		BankIDVerified bool `db:"bank_id_verified"`; IsActive bool `db:"is_active"`; CreatedAt time.Time `db:"created_at"`; UpdatedAt time.Time `db:"updated_at"`
	}{ID: u.ID, OrgID: u.OrgID, Email: u.Email, PasswordHash: passwordHash, FirstName: u.FirstName, LastName: u.LastName, Phone: sql.NullString{String: u.Phone, Valid: u.Phone != ""}, Role: string(u.Role), BankIDVerified: u.BankIDVerified, IsActive: u.IsActive, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt}
	_, err := r.db.NamedExecContext(ctx, q, row)
	if err != nil { return fmt.Errorf("create user: %w", err) }
	return nil
}

func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	var row userRow
	if err := r.db.GetContext(ctx, &row, `SELECT * FROM users WHERE id = $1 AND is_active = true`, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) { return nil, ErrUserNotFound }
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return row.toDomain(), nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	var row userRow
	if err := r.db.GetContext(ctx, &row, `SELECT * FROM users WHERE LOWER(email) = LOWER($1) AND is_active = true`, email); err != nil {
		if errors.Is(err, sql.ErrNoRows) { return nil, ErrUserNotFound }
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	return row.toDomain(), nil
}

func (r *userRepository) GetPasswordHash(ctx context.Context, userID uuid.UUID) (string, error) {
	var hash string
	if err := r.db.GetContext(ctx, &hash, `SELECT password_hash FROM users WHERE id = $1 AND is_active = true`, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) { return "", ErrUserNotFound }
		return "", fmt.Errorf("get password hash: %w", err)
	}
	return hash, nil
}

func (r *userRepository) FindByOrgID(ctx context.Context, orgID uuid.UUID, filters ports.UserFilters) ([]*user.User, int, error) {
	var sb strings.Builder
	args := []interface{}{orgID}
	argIdx := 2
	sb.WriteString(`SELECT * FROM users WHERE org_id = $1`)
	if filters.Role != "" { fmt.Fprintf(&sb, ` AND role = $%d`, argIdx); args = append(args, filters.Role); argIdx++ }
	limit := filters.Limit; if limit <= 0 { limit = 50 }
	fmt.Fprintf(&sb, ` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, filters.Offset)
	var rows []userRow
	if err := r.db.SelectContext(ctx, &rows, sb.String(), args...); err != nil { return nil, 0, err }
	users := make([]*user.User, len(rows))
	for i, row := range rows { row := row; users[i] = row.toDomain() }
	return users, len(rows), nil
}

func (r *userRepository) Update(ctx context.Context, u *user.User) error {
	const q = `UPDATE users SET first_name = :first_name, last_name = :last_name, phone = :phone, role = :role, updated_at = :updated_at WHERE id = :id`
	row := struct {
		ID uuid.UUID `db:"id"`; FirstName string `db:"first_name"`; LastName string `db:"last_name"`
		Phone sql.NullString `db:"phone"`; Role string `db:"role"`; UpdatedAt time.Time `db:"updated_at"`
	}{ID: u.ID, FirstName: u.FirstName, LastName: u.LastName, Phone: sql.NullString{String: u.Phone, Valid: u.Phone != ""}, Role: string(u.Role), UpdatedAt: time.Now().UTC()}
	result, err := r.db.NamedExecContext(ctx, q, row)
	if err != nil { return fmt.Errorf("update user: %w", err) }
	n, _ := result.RowsAffected()
	if n == 0 { return ErrUserNotFound }
	return nil
}

func (r *userRepository) Deactivate(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `UPDATE users SET is_active = false, updated_at = NOW() WHERE id = $1`, id)
	if err != nil { return err }
	n, _ := result.RowsAffected()
	if n == 0 { return ErrUserNotFound }
	return nil
}

func (r *userRepository) CreateApartment(ctx context.Context, a *user.Apartment) error {
	const q = `INSERT INTO apartments (id, org_id, building_id, apartment_number, floor, room_count, lease_code, is_active, created_at, updated_at) VALUES (:id, :org_id, :building_id, :apartment_number, :floor, :room_count, :lease_code, :is_active, :created_at, :updated_at)`
	row := struct {
		ID uuid.UUID `db:"id"`; OrgID uuid.UUID `db:"org_id"`; BuildingID uuid.UUID `db:"building_id"`
		ApartmentNumber string `db:"apartment_number"`; Floor sql.NullInt64 `db:"floor"`; RoomCount int `db:"room_count"`
		LeaseCode string `db:"lease_code"`; IsActive bool `db:"is_active"`; CreatedAt time.Time `db:"created_at"`; UpdatedAt time.Time `db:"updated_at"`
	}{ID: a.ID, OrgID: a.OrgID, BuildingID: a.BuildingID, ApartmentNumber: a.ApartmentNumber, Floor: sql.NullInt64{Int64: int64(a.Floor), Valid: a.Floor != 0}, RoomCount: a.RoomCount, LeaseCode: a.LeaseCode, IsActive: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	_, err := r.db.NamedExecContext(ctx, q, row)
	return err
}

func (r *userRepository) FindApartmentByLeaseCode(ctx context.Context, leaseCode string) (*user.Apartment, error) {
	var row apartmentRow
	if err := r.db.GetContext(ctx, &row, `SELECT * FROM apartments WHERE lease_code = $1 AND is_active = true`, leaseCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) { return nil, ErrApartmentNotFound }
		return nil, fmt.Errorf("find apartment by lease code: %w", err)
	}
	return row.toDomain(), nil
}

func (r *userRepository) FindApartmentByUserID(ctx context.Context, userID uuid.UUID) (*user.Apartment, error) {
	var row apartmentRow
	if err := r.db.GetContext(ctx, &row, `SELECT * FROM apartments WHERE user_id = $1 AND is_active = true`, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) { return nil, ErrApartmentNotFound }
		return nil, fmt.Errorf("find apartment by user: %w", err)
	}
	return row.toDomain(), nil
}

func (r *userRepository) UpdateApartmentRoomCount(ctx context.Context, apartmentID uuid.UUID, roomCount int) error {
	if roomCount < 1 || roomCount > 3 { return fmt.Errorf("room count must be 1-3") }
	_, err := r.db.ExecContext(ctx, `UPDATE apartments SET room_count = $1, updated_at = NOW() WHERE id = $2`, roomCount, apartmentID)
	return err
}

func (r *userRepository) AssignUserToApartment(ctx context.Context, userID, apartmentID uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `UPDATE apartments SET user_id = $1, updated_at = NOW() WHERE id = $2 AND user_id IS NULL`, userID, apartmentID)
	if err != nil { return fmt.Errorf("assign user to apartment: %w", err) }
	n, _ := result.RowsAffected()
	if n == 0 { return ErrApartmentAlreadyOccupied }
	return nil
}

func (r *userRepository) SetBankIDVerified(ctx context.Context, userID uuid.UUID, verifiedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET bank_id_verified = true, bank_id_last_verified_at = $1, updated_at = NOW() WHERE id = $2`, verifiedAt.UTC(), userID)
	return err
}

func hashToken(token string) string { h := sha256.Sum256([]byte(token)); return hex.EncodeToString(h[:]) }

func (r *userRepository) SaveRefreshToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at) VALUES ($1, $2, $3, $4, $5)`, uuid.New(), userID, hashToken(token), expiresAt.UTC(), time.Now().UTC())
	return err
}

func (r *userRepository) FindRefreshToken(ctx context.Context, token string) (*ports.RefreshTokenRecord, error) {
	var row struct {
		UserID uuid.UUID `db:"user_id"`; ExpiresAt time.Time `db:"expires_at"`; RevokedAt sql.NullTime `db:"revoked_at"`
	}
	if err := r.db.GetContext(ctx, &row, `SELECT user_id, expires_at, revoked_at FROM refresh_tokens WHERE token_hash = $1`, hashToken(token)); err != nil {
		if errors.Is(err, sql.ErrNoRows) { return nil, ErrTokenNotFound }
		return nil, err
	}
	rec := &ports.RefreshTokenRecord{UserID: row.UserID, ExpiresAt: row.ExpiresAt}
	if row.RevokedAt.Valid { t := row.RevokedAt.Time; rec.RevokedAt = &t }
	return rec, nil
}

func (r *userRepository) RevokeRefreshToken(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE refresh_tokens SET revoked_at = NOW() WHERE token_hash = $1`, hashToken(token))
	return err
}

func (r *userRepository) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `UPDATE refresh_tokens SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

var (
	ErrUserNotFound             = errors.New("user not found")
	ErrApartmentNotFound        = errors.New("apartment not found")
	ErrApartmentAlreadyOccupied = errors.New("apartment is already occupied")
	ErrTokenNotFound            = errors.New("refresh token not found")
)
