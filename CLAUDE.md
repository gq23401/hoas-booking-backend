# HOAS Booking — CLAUDE.md

This file is read by Claude Code at the start of every session.
Read this entire file before touching any code.

## What This Project Is

Multi-tenant SaaS booking system for Finnish residential housing companies.
Tenants book shared laundry machines and saunas in their apartment building.

**Live spec:**
- 3 washers + 1 dryer per laundry room (4 bookable machines)
- Slots are 1 hour each
- Laundry: 07:00–22:00 Helsinki time, 7 days, 8 weeks advance booking
- Sauna: 16:00–21:00 Helsinki time, Thursday–Sunday only, 2 weeks advance
- Laundry quota: 1 room = 5 slots/month, 2 rooms = 10, 3 rooms = 15
- Sauna quota: 1 slot per tenant per week (Monday reset), NO refund on cancel
- Laundry cancel <12h before = slot still consumed from quota
- No-show: 15 min after slot start → auto-released, waitlist notified
- Quota resets: 1st of each month at 00:00 UTC

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.22, Echo v4, sqlx, PostgreSQL 16 |
| Auth | JWT (15min access / 7d refresh) + bcrypt + Mock Bank ID |
| Background jobs | robfig/cron v3 |
| Frontend | Next.js 15, Tailwind CSS |
| Deployment | Fly.io + Docker |

## Architecture Rules

- Domain files NEVER import framework packages
- Application files NEVER import infrastructure packages  
- main.go is the ONLY file allowed to import everything
- One use case per file in application/booking/
- Interfaces live in application/ports/
- org_id on EVERY query — multi-tenancy is not optional
- Slot times generated in Europe/Helsinki timezone

## Key Business Rules

1. Quota check before availability check — fail fast
2. org_id on every query — SaaS isolation boundary
3. Sauna only Thu–Sun — validate day before availability
4. No-show grace = 15 min
5. Sauna cancel always consumes quota — no refund
6. Laundry cancel <12h = quota consumed
7. DB exclusion constraint is last defense against double-booking
8. Week starts Monday (MondayOf() in domain/booking/entity.go)

## Running Locally

```bash
docker compose up -d postgres redis
cp .env.example .env
# Edit .env: set DB_PASSWORD=hoas_secret
go mod tidy
go run ./cmd/api
```

Health check: `curl localhost:8080/health`

## Seed Data for Testing

```sql
INSERT INTO organizations (id, name, slug) VALUES ('00000000-0000-0000-0000-000000000001', 'Test Housing', 'test') ON CONFLICT DO NOTHING;
INSERT INTO buildings (id, org_id, name, address) VALUES ('00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'Building A', 'Testikatu 1') ON CONFLICT DO NOTHING;
INSERT INTO apartments (id, org_id, building_id, apartment_number, room_count, lease_code, is_active) VALUES ('00000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002', 'A1', 2, 'LEASE-001', true) ON CONFLICT DO NOTHING;
INSERT INTO laundry_rooms (id, building_id, org_id, name, floor, is_active) VALUES ('00000000-0000-0000-0000-000000000010', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'Staircase B', 1, true) ON CONFLICT DO NOTHING;
INSERT INTO machines (id, laundry_room_id, org_id, name, type, status) VALUES ('00000000-0000-0000-0000-000000000011', '00000000-0000-0000-0000-000000000010', '00000000-0000-0000-0000-000000000001', 'Washing Machine 1', 'washer', 'active') ON CONFLICT DO NOTHING;
```

Then register at POST /api/v1/auth/register with lease_code: "LEASE-001"
Grant access: INSERT INTO tenant_laundry_room_access ... (see README)

## What Claude Should NOT Do

- Do not add GORM — we use sqlx intentionally
- Do not move business logic into HTTP handlers
- Do not import infrastructure/postgres from anything except cmd/api/main.go
- Do not skip org_id in queries
- Do not hardcode timezones as UTC offsets — use time.LoadLocation("Europe/Helsinki")
