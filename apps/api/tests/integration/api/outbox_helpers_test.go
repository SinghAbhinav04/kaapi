package api_test

import (
	"regexp"
	"strings"
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
		if strings.EqualFold(string(recipient), strings.TrimSpace(to)) { // addresses are stored lower-cased
			out = append(out, m)
		}
	}
	return out
}

var codeRe = regexp.MustCompile(`\b\d{6}\b`)

// emailedCode reads the newest sign-in code emailed to an address, as the person would from
// their inbox: the API never returns codes.
func emailedCode(t *testing.T, s *apitest.Server, kind domain.OutboxKind, to string) string {
	t.Helper()
	mails := outboxTo(t, s, kind, to)
	if len(mails) == 0 {
		t.Fatalf("no %s email to %s", kind, to)
	}
	code := codeRe.FindString(mails[0].Body)
	if code == "" {
		t.Fatalf("no code in the %s email", kind)
	}
	return code
}
