// Package audit provides a real-time listener for audit_entries inserts
// using PostgreSQL's LISTEN/NOTIFY mechanism.
package audit

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"
	"strings"
	"fmt"

	"github.com/lib/pq"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/db"
)

// Entry mirrors the GraphQL AuditEntry type.
type Entry struct {
	ID        string
	TenantID  string
	SessionID string
	ToolName  string
	Decision  string
	PrevHash  string
	RowHash   string
	CreatedAt time.Time
}

// Hub manages subscribers and fans out new audit entries to them.
type Hub struct {
	db       *sql.DB
	listener *pq.Listener

	mu   sync.RWMutex
	subs map[chan *Entry]string // channel -> tenantID filter
}

// NewHub creates a Hub and starts listening on audit_entries_channel.
func NewHub(dbURL string, database *sql.DB) (*Hub, error) {
	pqListener := pq.NewListener(dbURL, 10*time.Second, time.Minute, func(ev pq.ListenerEventType, err error) {
		if err != nil {
			log.Printf("audit hub: %v", err)
		}
	})

	if err := pqListener.Listen("audit_entries_channel"); err != nil {
		return nil, err
	}

	return &Hub{
		db:       database,
		listener: pqListener,
		subs:     make(map[chan *Entry]string),
	}, nil
}

// Start runs the notification loop. It blocks until ctx is cancelled.
func (h *Hub) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			_ = h.listener.Close()
			h.mu.Lock()
			for ch := range h.subs {
				close(ch)
			}
			h.subs = make(map[chan *Entry]string)
			h.mu.Unlock()
			return
		case notification := <-h.listener.Notify:
			if notification == nil {
				continue
			}
			entry, err := h.fetchEntry(ctx, notification.Extra)
			if err != nil {
				log.Printf("audit hub: fetch %s: %v", notification.Extra, err)
				continue
			}
			h.broadcast(entry)
			log.Printf("audit hub: received notification for id %s", notification.Extra)
		}
	}
}

// Subscribe registers a subscriber filtered by tenantID (empty = all).
func (h *Hub) Subscribe(tenantID string) chan *Entry {
	log.Printf("audit hub: new subscriber for tenant %q", tenantID)
	ch := make(chan *Entry, 16)
	h.mu.Lock()
	h.subs[ch] = tenantID
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (h *Hub) Unsubscribe(ch chan *Entry) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.subs[ch]; ok {
		delete(h.subs, ch)
		close(ch)
	}
}

func (h *Hub) broadcast(entry *Entry) {
	log.Printf("audit hub: broadcasting entry %s for tenant %s", entry.ID, entry.TenantID)
	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch, tenantFilter := range h.subs {
		if tenantFilter != "" && tenantFilter != entry.TenantID {
			continue
		}
		select {
		case ch <- entry:
		default:
			// Slow subscriber: skip. It will resync on reconnect.
		}
	}
}

func (h *Hub) fetchEntry(ctx context.Context, payload string) (*Entry, error) {
	parts := strings.SplitN(payload, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid notification payload: %q", payload)
	}
	tenantID, id := parts[0], parts[1]

	// Inject the tenant ID into the context so that db.WithTenantTx can
	// set the RLS session variable before querying.
	tenantCtx := db.WithTenantID(ctx, tenantID)

	var e Entry
	var toolName sql.NullString
	err := db.WithTenantTx(tenantCtx, h.db, func(tx *sql.Tx) error {
		return tx.QueryRowContext(tenantCtx, `
			SELECT id, tenant_id, session_id, tool_name, decision,
			       prev_hash, row_hash, created_at
			FROM audit_entries WHERE id = $1`, id).
			Scan(&e.ID, &e.TenantID, &e.SessionID, &toolName,
				&e.Decision, &e.PrevHash, &e.RowHash, &e.CreatedAt)
	})
	if err != nil {
		return nil, err
	}
	if toolName.Valid {
		e.ToolName = toolName.String
	}
	return &e, nil
}