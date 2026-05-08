# RentSpace Backend

Go REST API for RentSpace — a short-term space rental marketplace for Thailand.

**Frontend repo:** https://github.com/tortrakul-won/rentspace-frontend

---

## Stack

Go, chi, sqlc, pgx, PostgreSQL, golang-migrate

---

## Project Structure

```
rentspace-backend/
├── cmd/api/
│   └── main.go                  entry point — loads .env, connects DB, starts server
├── internal/
│   ├── config/
│   │   └── config.go            reads SERVER_PORT, DATABASE_URL, JWT_SECRET from env
│   ├── middleware/
│   │   └── auth.go              JWT validation middleware + claims context helpers
│   ├── store/
│   │   ├── connect.go           opens *sql.DB connection via pgx stdlib driver
│   │   ├── db.go                sqlc-generated DBTX interface (do not edit)
│   │   ├── models.go            sqlc-generated structs: User, Profile, Space, Booking (do not edit)
│   │   ├── querier.go           sqlc-generated query interface (do not edit)
│   │   ├── users.sql.go         sqlc-generated user queries (do not edit)
│   │   ├── profiles.sql.go      sqlc-generated profile queries (do not edit)
│   │   ├── spaces.sql.go        sqlc-generated space queries (do not edit)
│   │   └── bookings.sql.go      sqlc-generated booking queries (do not edit)
│   ├── handler/
│   │   ├── dto.go               all API request/response structs
│   │   ├── auth.go              register, login, switch-profile, add-profile, current-user
│   │   ├── health.go            GET /health
│   │   ├── spaces.go            spaces CRUD handlers
│   │   ├── bookings.go          bookings handlers + overlap check
│   │   ├── respond.go           JSON() and Error() response helpers
│   │   └── util.go              UUID parsing helper
│   └── api/
│       └── routes.go            all routes — see file for per-endpoint comments
├── db/
│   ├── migrations/
│   │   ├── 000001_init.up.sql   creates users, profiles, spaces, bookings tables
│   │   └── 000001_init.down.sql drops all tables and types
│   └── queries/
│       ├── users.sql            SQL source for sqlc (user queries)
│       ├── profiles.sql         SQL source for sqlc (profile queries)
│       ├── spaces.sql           SQL source for sqlc (space queries)
│       └── bookings.sql         SQL source for sqlc (booking queries)
├── sqlc.yaml                    sqlc config — maps queries → generated Go code
└── .env.example                 template for required environment variables
```

---

## Setup

### Prerequisites
- Go 1.25+
- PostgreSQL running locally
- [golang-migrate CLI](https://github.com/golang-migrate/migrate)

```bash
# copy and fill in your credentials
cp .env.example .env

# run migrations
migrate -path db/migrations -database "$DATABASE_URL" up

# start server
go run ./cmd/api     # http://localhost:8080
```

---

## Environment Variables

| Variable | Description |
|---|---|
| `SERVER_PORT` | HTTP server port (default: `8080`) |
| `DATABASE_URL` | PostgreSQL connection string |
| `JWT_SECRET` | Secret key for JWT signing |

---

## API Overview

All protected routes require `Authorization: Bearer <token>`.

### Auth (public)

| Method | Path | Description |
|---|---|---|
| POST | `/api/v1/auth/register` | Create account + first profile, returns JWT |
| POST | `/api/v1/auth/login` | Verify credentials, returns JWT + all profiles |

### Auth (protected)

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/auth/me` | Current user, all profiles, active profile ID |
| POST | `/api/v1/auth/switch-profile` | Swap active profile, returns new JWT |
| POST | `/api/v1/auth/profiles` | Add a second profile (owner or renter) |

### Spaces (protected)

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/spaces` | List all active spaces |
| GET | `/api/v1/spaces/{id}` | Get a space |
| POST | `/api/v1/spaces` | Create a space — owner profile only |
| PUT | `/api/v1/spaces/{id}` | Update a space — owner profile only |
| DELETE | `/api/v1/spaces/{id}` | Deactivate a space — owner profile only |

### Bookings (protected)

| Method | Path | Description |
|---|---|---|
| POST | `/api/v1/bookings` | Create a booking — renter profile only |
| GET | `/api/v1/bookings/{id}` | Get a booking |
| PATCH | `/api/v1/bookings/{id}/status` | Update booking status |
| GET | `/api/v1/spaces/{id}/bookings` | List all bookings for a space |

### System

| Method | Path | Description |
|---|---|---|
| GET | `/health` | Server liveness check |
