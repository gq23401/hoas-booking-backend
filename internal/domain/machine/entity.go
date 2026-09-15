package machine

import (
	"time"
	"github.com/google/uuid"
)

type Type string
const (TypeWasher Type = "washer"; TypeDryer Type = "dryer")

type Status string
const (StatusActive Status = "active"; StatusMaintenance Status = "maintenance"; StatusRetired Status = "retired")

type Machine struct {
	ID uuid.UUID; LaundryRoomID uuid.UUID; OrgID uuid.UUID
	Name string; Type Type; Status Status; CreatedAt time.Time; UpdatedAt time.Time
}

func (m *Machine) IsBookable() bool { return m.Status == StatusActive }

type LaundryRoom struct {
	ID uuid.UUID; BuildingID uuid.UUID; OrgID uuid.UUID
	Name string; Floor int; IsActive bool; Machines []*Machine
}

type MaintenanceWindow struct {
	ID uuid.UUID; MachineID uuid.UUID; OrgID uuid.UUID
	StartsAt time.Time; EndsAt time.Time; Reason string; CreatedBy uuid.UUID; CreatedAt time.Time
}

func (w *MaintenanceWindow) Overlaps(slotStart, slotEnd time.Time) bool {
	return w.StartsAt.Before(slotEnd) && w.EndsAt.After(slotStart)
}
