package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const eventChannel = "list_events"

// hub fans Postgres notifications out to connected SSE clients. Notifications
// are best effort: a client that misses one (slow, or reconnecting) catches up
// with GET /api/sync, so a full client buffer drops the event rather than block.
type hub struct {
	mu   sync.Mutex
	subs map[chan string]struct{}
}

func newHub() *hub { return &hub{subs: map[chan string]struct{}{}} }

func (h *hub) subscribe() chan string {
	ch := make(chan string, 64)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *hub) unsubscribe(ch chan string) {
	h.mu.Lock()
	delete(h.subs, ch)
	h.mu.Unlock()
}

func (h *hub) publish(payload string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- payload:
		default:
		}
	}
}

// listen holds one dedicated connection LISTENing on the events channel until ctx
// ends. It is not taken from the pool, so closing the pool never waits on it.
func (h *hub) listen(ctx context.Context, pool *pgxpool.Pool) {
	backoff := time.Second
	for ctx.Err() == nil {
		err := h.listenOnce(ctx, pool)
		if ctx.Err() != nil {
			return
		}
		log.Printf("events: listener stopped: %v; retrying in %s", err, backoff)
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

func (h *hub) listenOnce(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pgx.ConnectConfig(ctx, pool.Config().ConnConfig)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(ctx, "LISTEN "+eventChannel); err != nil {
		return err
	}
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		h.publish(n.Payload)
	}
}

// events streams item changes as server-sent events:
//
//	event: item
//	data: {"listId":"…","itemId":"…","version":42}
func (s *server) events(w http.ResponseWriter, r *http.Request) {
	if _, err := s.member(r.Context()); err != nil {
		responseError(w, r, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	// Subscribe before replying so nothing between the response and the first read is lost.
	ch := s.hub.subscribe()
	defer s.hub.unsubscribe(ch)

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no")
	fmt.Fprint(w, "retry: 3000\n\n: connected\n\n")
	flusher.Flush()

	beat := time.NewTicker(25 * time.Second)
	defer beat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case p := <-ch:
			fmt.Fprintf(w, "event: item\ndata: %s\n\n", p)
			flusher.Flush()
		case <-beat.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
