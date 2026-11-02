package enquiry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kaapi/api/internal/ai"
	"kaapi/api/internal/domain"
	"kaapi/api/internal/inventory"
	"kaapi/api/internal/platform/idgen"
	"kaapi/api/internal/platform/money"
)

// Who the sales team is for event enquiries. In production this comes from the owner of the
// Sales lane; the fallback keeps drafts sensible if that account is missing.
const eventTeamFallback = "our events team on +91 90000 00004"

// templates are used when no model is configured or the model is unavailable. They follow
// the same placeholder rules as AI drafts, so the same server-side filling applies.
var templates = map[domain.Category]string{
	domain.CategoryRoom: `Hi {{GUEST_NAME}},

Thank you for writing to Kaapi Estate. Here's what we can offer:

{{QUOTE}}

If you'd like, we can hold a room for you while you decide. Just reply and we'll take care of it.

{{SIGNATURE}}`,
	domain.CategoryWedding: `Hi {{GUEST_NAME}},

Thank you for thinking of Kaapi Estate for your celebration. Our Estate Lawn hosts up to 400 guests, and the Valley Deck suits smaller ceremonies.

We'd love to understand your plans and share options and a quote. You can reach {{EVENT_TEAM_CONTACT}}, or reply with a good time to call.

{{SIGNATURE}}`,
	domain.CategoryOffsite: `Hi {{GUEST_NAME}},

Thank you for considering Kaapi Estate for your team. The Planters' Hall seats up to 80, and we can add estate walks, coffee tastings and group dining.

Share your dates and group size and we'll send a proposal. You can also reach {{EVENT_TEAM_CONTACT}}.

{{SIGNATURE}}`,
	domain.CategoryComplaint: `Hi {{GUEST_NAME}},

Thank you for telling us, and I'm sorry. This isn't the stay we want for any guest.

Our General Manager has your message and will personally follow up with you within the day.

{{SIGNATURE}}`,
	domain.CategoryOther: `Hi {{GUEST_NAME}},

Thank you for your message. A member of our team will get back to you shortly.

{{SIGNATURE}}`,
}

// querier is what both a pool and a transaction can do; reads that happen before a
// transaction opens use the pool.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// prose is a draft's wording before its placeholders are filled.
type prose struct {
	template string
	author   string // AI or TEMPLATE
	flag     string
	staff    Staff
}

// writeProse asks the model for the wording (or picks a template) and guards it. It makes
// a network call, so callers run it before opening a transaction, never inside one.
func (s *Service) writeProse(ctx context.Context, q querier, message, senderName string, category domain.Category,
	ex ai.Extracted, owner *uuid.UUID) (prose, error) {
	staff, err := staffFor(ctx, q, owner)
	if err != nil {
		return prose{}, err
	}
	p := prose{staff: staff, author: "AI"}
	if s.provider != nil {
		facts, err := factsFor(ctx, q)
		if err != nil {
			return prose{}, err
		}
		text, derr := s.provider.Draft(ctx, ai.DraftInput{Message: ai.Redact(message, senderName), Category: category,
			Extracted: ex, Facts: facts, StaffName: staff.Name, Today: s.today()})
		if derr == nil {
			p.template, p.flag = ai.Guard(text)
		} else if errors.Is(derr, context.Canceled) {
			return prose{}, derr
		}
	}
	if p.template == "" || !strings.Contains(p.template, ai.PlaceholderSignature) {
		p.template, p.author = templates[category], "TEMPLATE"
		if s.provider != nil {
			p.flag = "AI draft unavailable or off-format, so this is a template"
		}
	}
	return p, nil
}

// storeDraft fills the placeholders from live data and saves the next draft version.
func (s *Service) storeDraft(ctx context.Context, tx pgx.Tx, enquiryID uuid.UUID, version int, p prose, message, senderName string,
	category domain.Category, ex ai.Extracted) error {
	rendered, quote, err := s.render(ctx, tx, p.template, message, senderName, category, ex, p.staff)
	if err != nil {
		return err
	}
	return s.saveDraft(ctx, tx, enquiryID, version, p.template, rendered, quote, p.author, nil, p.flag)
}

// render replaces the placeholders. Prices and availability come only from here.
func (s *Service) render(ctx context.Context, tx pgx.Tx, template, message, senderName string, category domain.Category,
	ex ai.Extracted, staff Staff) (string, []byte, error) {
	quoteText, quote := "", []byte(nil)
	if strings.Contains(template, ai.PlaceholderQuote) && category == domain.CategoryRoom {
		quoteText, quote = s.quote(ctx, message, ex)
	}
	if quoteText == "" {
		quoteText = "Let us know your dates and who's travelling, and we'll send prices and availability straight away."
	}
	eventTeam := eventTeamFallback
	var salesName, salesPhone *string
	if err := tx.QueryRow(ctx, `SELECT name, phone FROM users WHERE role = $1 AND is_active ORDER BY created_at LIMIT 1`,
		domain.RoleSales).Scan(&salesName, &salesPhone); err == nil && salesName != nil && salesPhone != nil {
		eventTeam = fmt.Sprintf("%s from our events team on %s", firstName(*salesName), *salesPhone)
	}
	r := strings.NewReplacer(
		ai.PlaceholderGuestName, greetingName(senderName),
		ai.PlaceholderQuote, quoteText,
		ai.PlaceholderEventTeam, eventTeam,
		ai.PlaceholderSignature, signature(staff),
	)
	return strings.TrimSpace(r.Replace(template)), quote, nil
}

// quote builds the availability block from the inventory service, the same numbers the
// website would show. It returns "" when the message doesn't give bookable dates.
func (s *Service) quote(ctx context.Context, message string, ex ai.Extracted) (string, []byte) {
	if ex.CheckIn == nil || ex.CheckOut == nil {
		return "", nil
	}
	party := domain.Party{Rooms: 1, Adults: defaultQuoteAdult}
	if ex.Rooms != nil {
		party.Rooms = *ex.Rooms
	}
	if ex.Adults != nil {
		party.Adults = *ex.Adults
	}
	if ex.Children != nil {
		party.Children = *ex.Children
	}
	stay := domain.Stay{CheckIn: *ex.CheckIn, CheckOut: *ex.CheckOut}
	opts, err := s.inventory.Search(ctx, inventory.SearchRequest{Stay: stay, Party: party})
	if err != nil {
		return "", nil
	}
	wantsVilla := strings.Contains(strings.ToLower(message), "villa")
	var lines []string
	type quoted struct {
		Room           string `json:"room"`
		RoomOnlyPaise  int64  `json:"room_only_paise"`
		BreakfastPaise int64  `json:"breakfast_paise,omitempty"`
		Left           int    `json:"left"`
	}
	var data []quoted
	for _, o := range opts {
		if o.Status != inventory.Available || len(o.Plans) == 0 || (wantsVilla && !strings.Contains(o.Slug, "villa")) {
			continue
		}
		q := quoted{Room: o.Name, RoomOnlyPaise: o.Plans[0].Quote.TotalPaise, Left: o.Left}
		line := fmt.Sprintf("• %s: %s room only", o.Name, money.FormatINR(q.RoomOnlyPaise))
		for _, p := range o.Plans {
			if p.Code == "BB-FLEX" {
				q.BreakfastPaise = p.Quote.TotalPaise
				line += fmt.Sprintf(", %s with breakfast", money.FormatINR(p.Quote.TotalPaise))
			}
		}
		if o.Left <= 2 {
			line += fmt.Sprintf(" (%d left)", o.Left)
		}
		lines = append(lines, line)
		data = append(data, q)
		if len(lines) == maxQuoteOptions {
			break
		}
	}
	who := fmt.Sprintf("%d adult%s", party.Adults, plural(party.Adults))
	if party.Children > 0 {
		who += fmt.Sprintf(", %d child%s", party.Children, map[bool]string{true: "ren", false: ""}[party.Children > 1])
	}
	head := fmt.Sprintf("For %s to %s (%d night%s, %s), totals including GST:",
		stay.CheckIn.Format("Mon 2 Jan"), stay.CheckOut.Format("Mon 2 Jan"), stay.Nights(), plural(stay.Nights()), who)
	if len(lines) == 0 {
		raw, _ := json.Marshal(map[string]any{"available": false})
		return fmt.Sprintf("We're fully booked online for %s to %s, I'm sorry. Reply and we'll suggest the nearest dates that work.",
			stay.CheckIn.Format("2 Jan"), stay.CheckOut.Format("2 Jan")), raw
	}
	raw, _ := json.Marshal(map[string]any{"check_in": stay.CheckIn.Format(time.DateOnly), "check_out": stay.CheckOut.Format(time.DateOnly),
		"party": party, "options": data, "priced_at": s.now().UTC()})
	return head + "\n" + strings.Join(lines, "\n"), raw
}

func (s *Service) saveDraft(ctx context.Context, tx pgx.Tx, enquiryID uuid.UUID, version int, template, rendered string,
	quote []byte, author string, authorID *uuid.UUID, flag string) error {
	if _, err := tx.Exec(ctx, `INSERT INTO enquiry_drafts (id, enquiry_id, version, body_template, body_rendered, quote, author, author_user_id, flagged_reason)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, idgen.New(), enquiryID, version, template, rendered, nullJSON(quote),
		draftAuthorColumn(author), authorID, nilIfEmpty(flag)); err != nil {
		return err
	}
	actor, typ := domain.ActorAI, domain.EventDraftCreated
	if author == "USER" {
		actor, typ = domain.ActorUser, domain.EventDraftEdited
	} else if author == "TEMPLATE" {
		actor = domain.ActorSystem
	}
	return addEvent(ctx, tx, enquiryID, typ, actor, authorID, nil, map[string]any{"version": version, "source": author})
}

// draftAuthorColumn maps our three sources onto the table's AI/USER check: templates are
// system-written, so they're stored as AI drafts and the event records the real source.
func draftAuthorColumn(author string) string {
	if author == "USER" {
		return "USER"
	}
	return "AI"
}

func staffFor(ctx context.Context, tx querier, owner *uuid.UUID) (Staff, error) {
	st := Staff{Name: "The Kaapi Estate team"}
	if owner == nil {
		return st, nil
	}
	err := tx.QueryRow(ctx, `SELECT id, name, role FROM users WHERE id = $1`, *owner).Scan(&st.ID, &st.Name, &st.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return Staff{Name: "The Kaapi Estate team"}, nil
	}
	return st, err
}

func factsFor(ctx context.Context, tx querier) ([]ai.Fact, error) {
	rows, err := tx.Query(ctx, `SELECT question, answer FROM kb_facts WHERE is_active ORDER BY category, key`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (ai.Fact, error) {
		var f ai.Fact
		return f, r.Scan(&f.Question, &f.Answer)
	})
}

var roleTitle = map[domain.Role]string{
	domain.RoleReservations: "Reservations", domain.RoleSales: "Events & Sales", domain.RoleGM: "General Manager",
}

func signature(st Staff) string {
	if st.ID == uuid.Nil {
		return st.Name + "\nKaapi Estate"
	}
	return fmt.Sprintf("%s\n%s, Kaapi Estate", firstName(st.Name), roleTitle[st.Role])
}

func greetingName(sender string) string {
	n := firstName(strings.TrimSpace(sender))
	if n == "" || strings.EqualFold(n, "unknown") {
		return "there"
	}
	return n
}

func firstName(full string) string { return strings.SplitN(full, " ", 2)[0] }

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// storedExtracted is the JSON shape of enquiries.extracted.
type storedExtracted struct {
	CheckIn     *string         `json:"check_in,omitempty"`
	CheckOut    *string         `json:"check_out,omitempty"`
	Adults      *int            `json:"adults,omitempty"`
	Children    *int            `json:"children,omitempty"`
	Rooms       *int            `json:"rooms,omitempty"`
	GuestCount  *int            `json:"guest_count,omitempty"`
	BudgetINR   *int64          `json:"budget_inr,omitempty"`
	BudgetBasis *ai.BudgetBasis `json:"budget_basis,omitempty"`
	EventMonth  *string         `json:"event_month,omitempty"`
}

func toStored(e ai.Extracted) storedExtracted {
	day := func(t *time.Time) *string {
		if t == nil {
			return nil
		}
		s := t.Format(time.DateOnly)
		return &s
	}
	return storedExtracted{CheckIn: day(e.CheckIn), CheckOut: day(e.CheckOut), Adults: e.Adults, Children: e.Children,
		Rooms: e.Rooms, GuestCount: e.GuestCount, BudgetINR: e.BudgetINR, BudgetBasis: e.BudgetBasis, EventMonth: e.EventMonth}
}

func fromStored(raw []byte) ai.Extracted {
	var s storedExtracted
	_ = json.Unmarshal(raw, &s)
	day := func(v *string) *time.Time {
		if v == nil {
			return nil
		}
		t, err := time.Parse(time.DateOnly, *v)
		if err != nil {
			return nil
		}
		return &t
	}
	return ai.Extracted{CheckIn: day(s.CheckIn), CheckOut: day(s.CheckOut), Adults: s.Adults, Children: s.Children,
		Rooms: s.Rooms, GuestCount: s.GuestCount, BudgetINR: s.BudgetINR, BudgetBasis: s.BudgetBasis, EventMonth: s.EventMonth}
}
