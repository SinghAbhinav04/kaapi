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
	"kaapi/api/internal/platform/pii"
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

I'm looking into this personally and will get back to you within the day with what we'll do to put it right.

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
}

// writeProse asks the model for the wording (or picks a template) and guards it. It makes
// a network call, so callers run it before opening a transaction, never inside one. The
// model writes as the lane's team: who sends the reply, and so signs it, isn't known yet.
func (s *Service) writeProse(ctx context.Context, q querier, message, senderName string, category domain.Category,
	ex ai.Extracted, lane domain.Lane) (prose, error) {
	p := prose{author: "AI"}
	if s.provider != nil {
		facts, err := factsFor(ctx, q)
		if err != nil {
			return prose{}, err
		}
		rooms, err := roomsFor(ctx, q)
		if err != nil {
			return prose{}, err
		}
		offers, err := offersFor(ctx, q, s.today())
		if err != nil {
			return prose{}, err
		}
		text, derr := s.provider.Draft(ctx, ai.DraftInput{Message: ai.Redact(message, senderName), Category: category,
			Extracted: ex, Facts: facts, Rooms: rooms, Offers: offers, StaffName: voiceFor(lane), Today: s.today()})
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
	rendered, quote, err := s.render(ctx, tx, p.template, message, senderName, category, ex)
	if err != nil {
		return err
	}
	return s.saveDraft(ctx, tx, enquiryID, version, p.template, rendered, quote, p.author, nil, p.flag)
}

// render replaces the placeholders. Prices and availability come only from here. The
// signature is left out: Send adds the sign-off of whoever presses it.
func (s *Service) render(ctx context.Context, tx pgx.Tx, template, message, senderName string, category domain.Category,
	ex ai.Extracted) (string, []byte, error) {
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
		domain.RoleSales).Scan(pii.IntoNull(&salesName), pii.IntoNull(&salesPhone)); err == nil && salesName != nil && salesPhone != nil {
		eventTeam = fmt.Sprintf("%s from our events team on %s", firstName(*salesName), *salesPhone)
	}
	r := strings.NewReplacer(
		ai.PlaceholderGuestName, greetingName(senderName),
		ai.PlaceholderQuote, quoteText,
		ai.PlaceholderEventTeam, eventTeam,
		ai.PlaceholderSignature, "",
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
	opts = pickQuoted(opts, wantsVilla)
	type quoted struct {
		Room           string `json:"room"`
		RoomOnlyPaise  int64  `json:"room_only_paise"`
		BreakfastPaise int64  `json:"breakfast_paise,omitempty"`
		Left           int    `json:"left"`
	}
	var data []quoted
	for _, o := range opts {
		q := quoted{Room: o.Name, RoomOnlyPaise: o.Plans[0].Quote.TotalPaise, Left: o.Left}
		line := fmt.Sprintf("- %s: %s room only", o.Name, money.FormatINR(q.RoomOnlyPaise))
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
	}
	who := fmt.Sprintf("%d adult%s", party.Adults, plural(party.Adults))
	if party.Children > 0 {
		who += fmt.Sprintf(", %d child%s", party.Children, map[bool]string{true: "ren", false: ""}[party.Children > 1])
	}
	if party.Rooms > 1 {
		who += fmt.Sprintf(", %d rooms", party.Rooms)
	}
	head := fmt.Sprintf("For %s to %s (%d night%s, %s), totals including GST:",
		stay.CheckIn.Format("Mon 2 Jan"), stay.CheckOut.Format("Mon 2 Jan"), stay.Nights(), plural(stay.Nights()), who)
	if ex.Adults == nil {
		// The guest didn't say who's coming: say what the prices assume rather than look exact.
		head = fmt.Sprintf("For %s to %s (%d night%s), priced for %s (tell us if that's different), totals including GST:",
			stay.CheckIn.Format("Mon 2 Jan"), stay.CheckOut.Format("Mon 2 Jan"), stay.Nights(), plural(stay.Nights()), who)
	}
	if len(lines) == 0 {
		raw, _ := json.Marshal(map[string]any{"available": false})
		return fmt.Sprintf("We're fully booked online for %s to %s, I'm sorry. Reply and we'll suggest the nearest dates that work.",
			stay.CheckIn.Format("2 Jan"), stay.CheckOut.Format("2 Jan")), raw
	}
	raw, _ := json.Marshal(map[string]any{"check_in": stay.CheckIn.Format(time.DateOnly), "check_out": stay.CheckOut.Format(time.DateOnly),
		"party": party, "options": data, "priced_at": s.now().UTC()})
	return head + "\n" + strings.Join(lines, "\n"), raw
}

// pickQuoted chooses the options a quote shows: the first that are bookable, in the site's
// order, keeping a place for the first villa so a guest weighing a villa sees its price (the
// draft often recommends one). A guest who asked about villas sees only villas.
func pickQuoted(opts []inventory.Option, villasOnly bool) []inventory.Option {
	var rooms, villas []inventory.Option
	for _, o := range opts {
		if o.Status != inventory.Available || len(o.Plans) == 0 {
			continue
		}
		if strings.Contains(o.Slug, "villa") {
			villas = append(villas, o)
		} else if !villasOnly {
			rooms = append(rooms, o)
		}
	}
	if villasOnly || len(villas) == 0 {
		return append(villas, rooms...)[:min(maxQuoteOptions, len(villas)+len(rooms))]
	}
	picked := rooms[:min(maxQuoteOptions-1, len(rooms))]
	return append(picked, villas[:min(maxQuoteOptions-len(picked), len(villas))]...)
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

// roomsFor describes every room type on sale, with its plans, for the draft. No prices.
func roomsFor(ctx context.Context, q querier) ([]ai.RoomFact, error) {
	rows, err := q.Query(ctx, `SELECT rt.name, rt.kind, rt.size_sqm, rt.bed, rt.view, rt.base_occupancy, rt.max_adults, rt.max_children,
			rt.amenities,
			coalesce((SELECT array_agg(rp.name || CASE WHEN rp.refundable THEN ' (' || rp.cancellation_policy || ')' ELSE ' (non-refundable)' END
				|| CASE WHEN rp.min_nights > 1 THEN ', min ' || rp.min_nights || ' nights' ELSE '' END ORDER BY rp.sort)
				FROM rate_plans rp WHERE rp.room_type_id = rt.id AND rp.is_active), '{}')
		FROM room_types rt WHERE rt.is_active ORDER BY rt.sort`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (ai.RoomFact, error) {
		var f ai.RoomFact
		var size, base, maxAdults, maxChildren int
		var bed, view string
		if err := r.Scan(&f.Name, &f.Kind, &size, &bed, &view, &base, &maxAdults, &maxChildren, &f.Amenities, &f.Plans); err != nil {
			return f, err
		}
		f.Description = fmt.Sprintf("%d sq m, %s bed, %s view", size, bed, view)
		f.Fits = fmt.Sprintf("%d adults included, up to %d adults and %d children", base, maxAdults, maxChildren)
		return f, nil
	})
}

// offersFor lists offers a guest can book today, with their conditions. Fixed-amount offers
// don't state the amount: money in the model's words is stripped, and totals come from the quote.
func offersFor(ctx context.Context, q querier, today time.Time) ([]ai.OfferFact, error) {
	rows, err := q.Query(ctx, `SELECT p.code, p.type, p.value, p.stay_from, p.stay_to, p.book_to, p.min_nights,
			coalesce((SELECT string_agg(rt.name, ', ' ORDER BY rt.sort) FROM room_types rt WHERE rt.id = ANY(p.room_type_ids)), '')
		FROM promo_codes p
		WHERE p.is_active AND p.book_from <= $1 AND p.book_to >= $1 AND p.stay_to >= $1
		  AND (p.max_uses IS NULL OR p.used_count < p.max_uses)
		ORDER BY p.stay_from, p.code`, today)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (ai.OfferFact, error) {
		var o ai.OfferFact
		var kind string
		var value int64
		var stayFrom, stayTo, bookTo time.Time
		var minNights int
		var rooms string
		if err := r.Scan(&o.Code, &kind, &value, &stayFrom, &stayTo, &bookTo, &minNights, &rooms); err != nil {
			return o, err
		}
		o.Summary = "a fixed amount off the stay (shown at checkout)"
		if kind == offerPercent {
			o.Summary = fmt.Sprintf("%d%% off", value/bpsPerPercent)
		}
		cond := fmt.Sprintf("Stays %s to %s, book by %s", stayFrom.Format("2 Jan 2006"), stayTo.Format("2 Jan 2006"), bookTo.Format("2 Jan 2006"))
		if minNights > 1 {
			cond += fmt.Sprintf(", minimum %d nights", minNights)
		}
		if rooms != "" {
			cond += ", " + rooms + " only"
		} else {
			cond += ", every room"
		}
		o.Conditions = cond
		return o, nil
	})
}

// voiceFor is who the model writes as, e.g. "the Reservations team". Complaints and the
// Unsorted lane belong to the General Manager.
func voiceFor(lane domain.Lane) string {
	if role := laneRole[lane]; role == domain.RoleGM {
		return "the General Manager"
	} else if title := role.Title(); title != "" {
		return "the " + title + " team"
	}
	return "the Kaapi Estate team"
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
