# HOAS Booking — Backend

Multi-tenant SaaS booking system for Finnish housing companies built with Go, Echo, PostgreSQL, and Clean Architecture.

**Frontend:** [hoas-booking-web](https://github.com/gq23401/hoas-booking-web)

## Stack
- **Go 1.22** + Echo v4 HTTP framework
- **PostgreSQL 16** with sqlx (raw SQL, no ORM)
- **JWT auth** (15min access / 7d refresh) + bcrypt + Mock Finnish Bank ID
- **Clean Architecture** — domain, application, infrastructure layers
- **Docker** + Fly.io deployment

## Run locally
```bash
docker compose up -d postgres redis
cp .env.example .env  # set DB_PASSWORD=hoas_secret
go mod tidy && go run ./cmd/api
curl http://localhost:8080/health
```

## Architecture
Dependencies point inward only. `main.go` is the only file allowed to import everything else.
