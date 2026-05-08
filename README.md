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
│   ├── store/
│   │   ├── connect.go           opens *sql.DB connection via pgx stdlib driver
│   │   ├── db.go                sqlc-generated DBTX interface (do not edit)
│   │   ├── models.go            sqlc-generated structs: Space, Booking, User (do not edit)
│   │   ├── querier.go           sqlc-generated query interface (do not edit)
│   │   ├── spaces.sql.go        sqlc-generated space queries (do not edit)
│   │   ├── bookings.sql.go      sqlc-generated booking queries (do not edit)
│   │   └── users.sql.go         sqlc-generated user queries (do not edit)
│   ├── handler/
│   │   ├── dto.go               API request/response structs (used for swagger + decoding)
│   │   ├── health.go            GET /health
│   │   ├── spaces.go            spaces CRUD handlers
│   │   ├── bookings.go          bookings handlers + overlap check
│   │   ├── respond.go           JSON() and Error() response helpers
│   │   └── util.go              UUID parsing helper
│   └── api/
│       └── routes.go            chi router — all routes registered here, CORS, middleware
├── db/
│   ├── migrations/
│   │   ├── 000001_init.up.sql   creates users, spaces, bookings tables + enums + indexes
│   │   └── 000001_init.down.sql drops all tables and types
│   └── queries/
│       ├── spaces.sql           SQL source for sqlc (spaces queries)
│       ├── bookings.sql         SQL source for sqlc (bookings queries)
│       └── users.sql            SQL source for sqlc (user queries)
├── docs/                        swagger-generated files (do not edit)
├── sqlc.yaml                    sqlc config — maps queries → generated Go code
├── .swaggo                      swag type overrides (maps sql.Null* → primitives)
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

## API Docs

Swagger UI is available at:
```
http://localhost:8080/swagger/index.html
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

| Method | Path | Description |
|---|---|---|
| GET | `/health` | Health check |
| GET | `/api/v1/spaces` | List all active spaces |
| GET | `/api/v1/spaces/:id` | Get a space |
| POST | `/api/v1/spaces` | Create a space |
| PUT | `/api/v1/spaces/:id` | Update a space |
| DELETE | `/api/v1/spaces/:id` | Deactivate a space |
| GET | `/api/v1/spaces/:id/bookings` | List bookings for a space |
| POST | `/api/v1/bookings` | Create a booking |
| GET | `/api/v1/bookings/:id` | Get a booking |
| PATCH | `/api/v1/bookings/:id/status` | Update booking status |
