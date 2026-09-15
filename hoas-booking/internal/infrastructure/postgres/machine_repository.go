package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	domainmachine "github.com/yourusername/hoas-booking/internal/domain/machine"
	"github.com/yourusername/hoas-booking/internal/application/ports"
)

type machineRepository struct { db *sqlx.DB }

func NewMachineRepository(db *sqlx.DB) ports.MachineRepository { return &machineRepository{db: db} }

type machineRow struct {
	ID uuid.UUID `db:"id"`; LaundryRoomID uuid.UUID `db:"laundry_room_id"`; OrgID uuid.UUID `db:"org_id"`
	Name string `db:"name"`; Type string `db:"type"`; Status string `db:"status"`; CreatedAt time.Time `db:"created_at"`; UpdatedAt time.Time `db:"updated_at"`
}
func (r *machineRow) toDomain() *domainmachine.Machine {
	return &domainmachine.Machine{ID: r.ID, LaundryRoomID: r.LaundryRoomID, OrgID: r.OrgID, Name: r.Name, Type: domainmachine.Type(r.Type), Status: domainmachine.Status(r.Status), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

type laundryRoomRow struct {
	ID uuid.UUID `db:"id"`; BuildingID uuid.UUID `db:"building_id"`; OrgID uuid.UUID `db:"org_id"`
	Name string `db:"name"`; Floor sql.NullInt64 `db:"floor"`; IsActive bool `db:"is_active"`
}
func (r *laundryRoomRow) toDomain() *domainmachine.LaundryRoom {
	return &domainmachine.LaundryRoom{ID: r.ID, BuildingID: r.BuildingID, OrgID: r.OrgID, Name: r.Name, Floor: int(r.Floor.Int64), IsActive: r.IsActive, Machines: []*domainmachine.Machine{}}
}

type maintenanceWindowRow struct {
	ID uuid.UUID `db:"id"`; MachineID uuid.UUID `db:"machine_id"`; OrgID uuid.UUID `db:"org_id"`
	StartsAt time.Time `db:"starts_at"`; EndsAt time.Time `db:"ends_at"`; Reason sql.NullString `db:"reason"`; CreatedBy uuid.UUID `db:"created_by"`; CreatedAt time.Time `db:"created_at"`
}
func (r *maintenanceWindowRow) toDomain() *domainmachine.MaintenanceWindow {
	mw := &domainmachine.MaintenanceWindow{ID: r.ID, MachineID: r.MachineID, OrgID: r.OrgID, StartsAt: r.StartsAt, EndsAt: r.EndsAt, CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt}
	if r.Reason.Valid { mw.Reason = r.Reason.String }
	return mw
}

func (r *machineRepository) FindByID(ctx context.Context, id uuid.UUID) (*domainmachine.Machine, error) {
	var row machineRow
	if err := r.db.GetContext(ctx, &row, `SELECT * FROM machines WHERE id = $1`, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) { return nil, ErrMachineNotFound }
		return nil, fmt.Errorf("find machine by id: %w", err)
	}
	return row.toDomain(), nil
}

func (r *machineRepository) FindByRoomID(ctx context.Context, roomID uuid.UUID) ([]*domainmachine.Machine, error) {
	var rows []machineRow
	if err := r.db.SelectContext(ctx, &rows, `SELECT * FROM machines WHERE laundry_room_id = $1 AND status != 'retired' ORDER BY CASE type WHEN 'washer' THEN 1 WHEN 'dryer' THEN 2 ELSE 3 END, name ASC`, roomID); err != nil {
		return nil, fmt.Errorf("find machines by room: %w", err)
	}
	machines := make([]*domainmachine.Machine, len(rows))
	for i, row := range rows { row := row; machines[i] = row.toDomain() }
	return machines, nil
}

func (r *machineRepository) FindRoomByID(ctx context.Context, roomID uuid.UUID) (*domainmachine.LaundryRoom, error) {
	var row laundryRoomRow
	if err := r.db.GetContext(ctx, &row, `SELECT * FROM laundry_rooms WHERE id = $1 AND is_active = true`, roomID); err != nil {
		if errors.Is(err, sql.ErrNoRows) { return nil, ErrRoomNotFound }
		return nil, fmt.Errorf("find room by id: %w", err)
	}
	return row.toDomain(), nil
}

func (r *machineRepository) FindRoomsByBuildingID(ctx context.Context, buildingID uuid.UUID) ([]*domainmachine.LaundryRoom, error) {
	var rows []laundryRoomRow
	if err := r.db.SelectContext(ctx, &rows, `SELECT * FROM laundry_rooms WHERE building_id = $1 AND is_active = true ORDER BY floor ASC NULLS LAST, name ASC`, buildingID); err != nil {
		return nil, fmt.Errorf("find rooms by building: %w", err)
	}
	rooms := make([]*domainmachine.LaundryRoom, len(rows))
	for i, row := range rows { row := row; rooms[i] = row.toDomain() }
	return rooms, nil
}

func (r *machineRepository) GrantAccess(ctx context.Context, userID, laundryRoomID, grantedBy uuid.UUID) error {
	const q = `INSERT INTO tenant_laundry_room_access (id, user_id, laundry_room_id, org_id, granted_by, granted_at) SELECT $1, $2, $3, org_id, $4, NOW() FROM laundry_rooms WHERE id = $3 ON CONFLICT (user_id, laundry_room_id) DO NOTHING`
	_, err := r.db.ExecContext(ctx, q, uuid.New(), userID, laundryRoomID, grantedBy)
	return err
}

func (r *machineRepository) RevokeAccess(ctx context.Context, userID, laundryRoomID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tenant_laundry_room_access WHERE user_id = $1 AND laundry_room_id = $2`, userID, laundryRoomID)
	return err
}

func (r *machineRepository) HasAccess(ctx context.Context, userID, laundryRoomID uuid.UUID) (bool, error) {
	var hasAccess bool
	if err := r.db.GetContext(ctx, &hasAccess, `SELECT EXISTS(SELECT 1 FROM tenant_laundry_room_access WHERE user_id = $1 AND laundry_room_id = $2)`, userID, laundryRoomID); err != nil {
		return false, fmt.Errorf("check access: %w", err)
	}
	return hasAccess, nil
}

func (r *machineRepository) GetAccessibleRooms(ctx context.Context, userID uuid.UUID) ([]*domainmachine.LaundryRoom, error) {
	const q = `SELECT lr.id AS room_id, lr.name AS room_name, lr.floor AS floor, lr.is_active AS is_active, lr.building_id AS building_id, lr.org_id AS org_id, m.id AS machine_id, m.name AS machine_name, m.type AS machine_type, m.status AS machine_status FROM laundry_rooms lr JOIN tenant_laundry_room_access tlra ON tlra.laundry_room_id = lr.id AND tlra.user_id = $1 LEFT JOIN machines m ON m.laundry_room_id = lr.id AND m.status != 'retired' WHERE lr.is_active = true ORDER BY lr.name ASC, m.name ASC`
	type joinRow struct {
		RoomID uuid.UUID `db:"room_id"`; RoomName string `db:"room_name"`; Floor sql.NullInt64 `db:"floor"`; IsActive bool `db:"is_active"`; BuildingID uuid.UUID `db:"building_id"`; OrgID uuid.UUID `db:"org_id"`
		MachineID uuid.NullUUID `db:"machine_id"`; MachineName sql.NullString `db:"machine_name"`; MachineType sql.NullString `db:"machine_type"`; MachineStatus sql.NullString `db:"machine_status"`
	}
	var rows []joinRow
	if err := r.db.SelectContext(ctx, &rows, q, userID); err != nil { return nil, fmt.Errorf("get accessible rooms: %w", err) }
	rooms := make(map[uuid.UUID]*domainmachine.LaundryRoom)
	roomOrder := make([]uuid.UUID, 0)
	for _, row := range rows {
		if _, exists := rooms[row.RoomID]; !exists {
			rooms[row.RoomID] = &domainmachine.LaundryRoom{ID: row.RoomID, BuildingID: row.BuildingID, OrgID: row.OrgID, Name: row.RoomName, Floor: int(row.Floor.Int64), IsActive: row.IsActive, Machines: []*domainmachine.Machine{}}
			roomOrder = append(roomOrder, row.RoomID)
		}
		if row.MachineID.Valid {
			rooms[row.RoomID].Machines = append(rooms[row.RoomID].Machines, &domainmachine.Machine{ID: row.MachineID.UUID, LaundryRoomID: row.RoomID, OrgID: row.OrgID, Name: row.MachineName.String, Type: domainmachine.Type(row.MachineType.String), Status: domainmachine.Status(row.MachineStatus.String)})
		}
	}
	result := make([]*domainmachine.LaundryRoom, len(roomOrder))
	for i, id := range roomOrder { result[i] = rooms[id] }
	return result, nil
}

func (r *machineRepository) CreateMaintenanceWindow(ctx context.Context, w *domainmachine.MaintenanceWindow) error {
	const q = `INSERT INTO machine_maintenance_windows (id, machine_id, org_id, starts_at, ends_at, reason, created_by, created_at) VALUES (:id, :machine_id, :org_id, :starts_at, :ends_at, :reason, :created_by, :created_at)`
	row := struct {
		ID uuid.UUID `db:"id"`; MachineID uuid.UUID `db:"machine_id"`; OrgID uuid.UUID `db:"org_id"`
		StartsAt time.Time `db:"starts_at"`; EndsAt time.Time `db:"ends_at"`; Reason sql.NullString `db:"reason"`; CreatedBy uuid.UUID `db:"created_by"`; CreatedAt time.Time `db:"created_at"`
	}{ID: w.ID, MachineID: w.MachineID, OrgID: w.OrgID, StartsAt: w.StartsAt, EndsAt: w.EndsAt, Reason: sql.NullString{String: w.Reason, Valid: w.Reason != ""}, CreatedBy: w.CreatedBy, CreatedAt: w.CreatedAt}
	_, err := r.db.NamedExecContext(ctx, q, row)
	return err
}

func (r *machineRepository) GetActiveMaintenanceWindows(ctx context.Context, machineID uuid.UUID, from, to time.Time) ([]*domainmachine.MaintenanceWindow, error) {
	const q = `SELECT * FROM machine_maintenance_windows WHERE machine_id = $1 AND starts_at < $3 AND ends_at > $2 ORDER BY starts_at ASC`
	var rows []maintenanceWindowRow
	if err := r.db.SelectContext(ctx, &rows, q, machineID, from, to); err != nil { return nil, err }
	windows := make([]*domainmachine.MaintenanceWindow, len(rows))
	for i, row := range rows { row := row; windows[i] = row.toDomain() }
	return windows, nil
}

func (r *machineRepository) UpdateMachineStatus(ctx context.Context, machineID uuid.UUID, status domainmachine.Status) error {
	result, err := r.db.ExecContext(ctx, `UPDATE machines SET status = $1, updated_at = NOW() WHERE id = $2`, string(status), machineID)
	if err != nil { return err }
	n, _ := result.RowsAffected()
	if n == 0 { return ErrMachineNotFound }
	return nil
}

var (
	ErrMachineNotFound = errors.New("machine not found")
	ErrRoomNotFound    = errors.New("laundry room not found")
)
