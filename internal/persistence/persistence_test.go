package persistence

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/thanhmaitran/compatlab/database"
)

func TestMemoryStore(t *testing.T) {
	storeContract(t, NewMemory(), "mem")
}

// The PostgreSQL contract test needs a database. It is skipped unless
// COMPATLAB_TEST_DATABASE_URL is set, for example:
//
//	postgres://compatlab:compatlab@localhost:5432/compatlab_test?sslmode=disable
func TestPostgresStore(t *testing.T) {
	url := os.Getenv("COMPATLAB_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("COMPATLAB_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pg, err := NewPostgres(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pg.Close()
	if err := pg.Migrate(ctx, database.Migrations, "migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := pg.Migrate(ctx, database.Migrations, "migrations"); err != nil {
		t.Fatalf("migrations must be idempotent: %v", err)
	}
	storeContract(t, pg, fmt.Sprintf("%d", time.Now().UnixNano()))
}
