// cmd/seed/main.go — dev-only DB reseed. Truncates all user data and inserts fake accounts.
// Run: source .env.dev && go run ./cmd/seed
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"

	"rentspace/backend/internal/store"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL not set")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Truncate all user-owned data
	if _, err := db.ExecContext(ctx, "TRUNCATE users CASCADE"); err != nil {
		log.Fatalf("truncate: %v", err)
	}
	fmt.Println("✓ truncated users (cascade)")

	q := store.New(db)

	type seedUser struct {
		email   string
		role    store.ProfileRole
		isAdmin bool
	}

	seeds := []seedUser{
		{"admin@rentspace.dev", store.ProfileRoleOwner, true},
		{"owner@rentspace.dev", store.ProfileRoleOwner, false},
		{"renter@rentspace.dev", store.ProfileRoleRenter, false},
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("Admin1234!"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("bcrypt: %v", err)
	}

	for _, s := range seeds {
		u, err := q.CreateUser(ctx, store.CreateUserParams{
			Email:        s.email,
			PasswordHash: string(hash),
		})
		if err != nil {
			log.Fatalf("create user %s: %v", s.email, err)
		}

		if s.isAdmin {
			if _, err := q.SetUserAdmin(ctx, store.SetUserAdminParams{ID: u.ID, IsAdmin: true}); err != nil {
				log.Fatalf("set admin %s: %v", s.email, err)
			}
		}

		_, err = q.CreateProfile(ctx, store.CreateProfileParams{
			UserID:       u.ID,
			Role:         s.role,
			ProfileName:  s.email,
			LegalNameTh:  "ผู้ใช้ทดสอบ",
			LegalNameEn:  "Seed User",
			Phone:        "081-000-0000",
			AddressLine1: "123 Dev Street",
			Subdistrict:  "Silom",
			District:     "Bang Rak",
			Province:     "Bangkok",
			PostalCode:   "10500",
			BranchNumber: "00000",
			TaxID:        sql.NullString{String: "1234567890123", Valid: true},
		})
		if err != nil {
			log.Fatalf("create profile %s: %v", s.email, err)
		}

		fmt.Printf("✓ %s  role=%s  admin=%v\n", s.email, s.role, s.isAdmin)
	}

	fmt.Println("\nDev DB reseeded. Password for all: Admin1234!")
}
