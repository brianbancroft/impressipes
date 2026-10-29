package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/brianbancroft/impressipies/apps/api/internal/db"
	"github.com/brianbancroft/impressipies/apps/api/internal/db/dbgen"
)

// testPool starts a throwaway Postgres 17 and migrates it. Tests are skipped
// with -short, or when Docker is unavailable.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping Postgres integration test in -short mode")
	}
	testcontainers.SkipIfProviderIsNotHealthy(t)
	ctx := context.Background()

	ctr, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("impressipes"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp").WithStartupTimeout(time.Minute)),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	testcontainers.CleanupContainer(t, ctr)

	url, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestMigrateIsIdempotent(t *testing.T) {
	pool := testPool(t)
	if err := db.Migrate(context.Background(), pool); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

func TestListItemSyncFlow(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	q := dbgen.New(pool)

	user, err := q.CreateUser(ctx, dbgen.CreateUserParams{DisplayName: "Mum", Role: "admin", AvatarColor: "#e11d48"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	list, err := q.CreateList(ctx, dbgen.CreateListParams{Name: "Costco", Emoji: "🛒", Colour: "#2563eb"})
	if err != nil {
		t.Fatalf("create list: %v", err)
	}
	cat, err := q.CreateListCategory(ctx, dbgen.CreateListCategoryParams{ListID: list.ID, Name: "Produce", Colour: "#16a34a"})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}

	id := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	add := dbgen.UpsertListItemParams{
		ID: id, ListID: list.ID, CategoryID: cat.ID, Name: "Bananas", QuantityText: "6", AddedBy: user.ID,
	}
	first, err := q.UpsertListItem(ctx, add)
	if err != nil {
		t.Fatalf("add item: %v", err)
	}
	// A retried outbox mutation with the same client id must not duplicate the item.
	if _, err := q.UpsertListItem(ctx, add); err != nil {
		t.Fatalf("retry add: %v", err)
	}
	items, err := q.ListItems(ctx, list.ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("items = %d, err = %v; want 1", len(items), err)
	}

	checked, err := q.SetItemChecked(ctx, dbgen.SetItemCheckedParams{ID: id, Checked: true, UserID: user.ID})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !checked.CheckedAt.Valid || checked.CheckedBy != user.ID {
		t.Fatalf("item not checked off by user: %+v", checked)
	}
	if checked.Version <= first.Version {
		t.Fatalf("version did not advance: %d -> %d", first.Version, checked.Version)
	}

	since, err := q.ItemsSince(ctx, first.Version)
	if err != nil || len(since) != 1 || since[0].ID != id {
		t.Fatalf("ItemsSince(%d) = %v, err = %v", first.Version, since, err)
	}
}

func TestSessionLookup(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	q := dbgen.New(pool)

	user, err := q.CreateUser(ctx, dbgen.CreateUserParams{DisplayName: "Dad", Role: "member", AvatarColor: "#2563eb"})
	if err != nil {
		t.Fatal(err)
	}
	expires := pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}
	if err := q.CreateSession(ctx, dbgen.CreateSessionParams{ID: "s1", UserID: user.ID, ExpiresAt: expires}); err != nil {
		t.Fatal(err)
	}
	got, err := q.GetSessionUser(ctx, "s1")
	if err != nil || got.ID != user.ID {
		t.Fatalf("session user = %+v, err = %v", got, err)
	}
	if err := q.DeleteUserSessions(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetSessionUser(ctx, "s1"); err == nil {
		t.Fatal("session survived DeleteUserSessions")
	}
}
