package api_test

import (
	"testing"

	"github.com/google/uuid"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/pii"
	"kaapi/api/tests/apitest"
)

// mailCopy is an outbox row as the app sees it: recipient and body decrypted.
type mailCopy struct {
	ID     uuid.UUID
	Status domain.OutboxStatus
	Body   string
}

// outboxTo returns the messages of one kind sent to an address, newest first. Recipients
// are encrypted at rest, so the match happens here, after decrypting, never in SQL.
func outboxTo(t *testing.T, s *apitest.Server, kind domain.OutboxKind, to string) []mailCopy {
	t.Helper()
	rows, err := s.Pool.Query(s.Ctx, `SELECT id, status, recipient, body FROM outbox_messages WHERE kind = $1 ORDER BY created_at DESC`, kind)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []mailCopy
	for rows.Next() {
		var m mailCopy
		var recipient pii.Text
		if err := rows.Scan(&m.ID, &m.Status, &recipient, pii.Into(&m.Body)); err != nil {
			t.Fatal(err)
		}
		if string(recipient) == to {
			out = append(out, m)
		}
	}
	return out
}
