// Package testdb starts one throwaway Postgres per test package with testcontainers-go.
package testdb

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

var URL string

// Start runs the tests of a package against a fresh database. Use it from TestMain:
// func TestMain(m *testing.M) { os.Exit(testdb.Start(m)) }
func Start(m *testing.M) int {
	ctx := context.Background()
	ctr, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("briefklar"), postgres.WithUsername("briefklar"), postgres.WithPassword("briefklar"),
		postgres.BasicWaitStrategies())
	if err != nil {
		fmt.Fprintln(os.Stderr, "starting postgres:", err)
		return 1
	}
	defer func() { _ = testcontainers.TerminateContainer(ctr) }()
	URL, err = ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return m.Run()
}

// Reset empties all tables (the schema stays).
func Reset(t *testing.T) {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), URL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(context.Background(), `TRUNCATE usage_day, app_user`); err != nil {
		t.Fatal(err)
	}
}
