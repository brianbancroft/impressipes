package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/brianbancroft/impressipies/apps/api/internal/db/dbgen"
	"github.com/brianbancroft/impressipies/apps/api/internal/family"
	"github.com/brianbancroft/impressipies/apps/api/internal/uuidx"
)

type viaKey struct{}

// clientVia is where a change came from, recorded in the activity log.
func clientVia(r *http.Request) string {
	switch v := r.Header.Get("X-Impressipies-Client"); v {
	case "ios", "watch", "web":
		return v
	}
	return "web"
}

// member resolves the signed-in member and requires the database to be configured.
func (s *server) member(ctx context.Context) (*family.Member, error) {
	m, f := s.current(ctx)
	if f != nil {
		return nil, fail(f.code, f.msg)
	}
	if s.pool == nil {
		return nil, fail(http.StatusServiceUnavailable, "database not configured")
	}
	return m, nil
}

// withTx runs fn in a transaction, committing when it returns nil.
func (s *server) withTx(ctx context.Context, fn func(q *dbgen.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func notFound(what string) error { return fail(http.StatusNotFound, what+" not found") }

// idOf parses a path id; a malformed id cannot exist, so it is a 404.
func idOf(s, what string) (pgtype.UUID, error) {
	u, ok := uuidx.Parse(s)
	if !ok {
		return u, notFound(what)
	}
	return u, nil
}

// orNotFound turns "no rows" into a 404 and passes other errors through.
func orNotFound(err error, what string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound(what)
	}
	return err
}

func (s *server) record(ctx context.Context, q *dbgen.Queries, m *family.Member, action string, list, recipe pgtype.UUID, detail map[string]any) error {
	_, err := s.recordRow(ctx, q, m, action, list, recipe, detail, batchOf(ctx), pgtype.Int8{})
	return err
}

func (s *server) recordRow(ctx context.Context, q *dbgen.Queries, m *family.Member, action string, list, recipe pgtype.UUID, detail map[string]any, batch pgtype.UUID, undoOf pgtype.Int8) (dbgen.Activity, error) {
	user, _ := uuidx.Parse(m.ID)
	via, _ := ctx.Value(viaKey{}).(string)
	if via == "" {
		via = "web"
	}
	if detail == nil {
		detail = map[string]any{}
	}
	b, err := json.Marshal(detail)
	if err != nil {
		return dbgen.Activity{}, err
	}
	return q.RecordActivity(ctx, dbgen.RecordActivityParams{
		UserID: user, Via: via, Action: action, ListID: list, RecipeID: recipe, Detail: b,
		BatchID: batch, UndoOf: undoOf,
	})
}

func numeric(f *float64) pgtype.Numeric {
	var n pgtype.Numeric
	if f != nil {
		_ = n.Scan(strconv.FormatFloat(*f, 'f', -1, 64))
	}
	return n
}

func floatOf(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	v, err := n.Float64Value()
	if err != nil || !v.Valid {
		return nil
	}
	return &v.Float64
}

func int4(i *int) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*i), Valid: true}
}

func intOf(i pgtype.Int4) *int {
	if !i.Valid {
		return nil
	}
	v := int(i.Int32)
	return &v
}

func text(p *string) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *p, Valid: true}
}

func textOf(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
