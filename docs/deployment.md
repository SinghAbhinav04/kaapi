# Kaapi Estate: deployment and how to try it

- **Live site:** https://kaapi-estate.vercel.app
- **Staff console:** https://kaapi-estate.vercel.app/staff/login
- **Code:** https://github.com/SinghAbhinav04/kaapi
- **Design document:** `Kaapi-Estate-TDD.pdf`

## Signing in

| Who | How |
|---|---|
| Reviewer (staff, Administrator) | `admin.kaapi@gmail.com` with the password given in the submission. This one account signs in with the password alone (`STAFF_PASSWORD_ONLY`), because reviewers don't have its mailbox. Every other staff account gets a 6-digit code by email after the password. |
| Guest | Any real email address on the site: a one-time code is emailed to it (codes are never shown on screen). Booking needs a signed-in guest, so the email is verified before payment. |

The Administrator sees every console page: Today, Inbox, Rates & allotment, Bookings, Outbox, Analytics, Offers, and Team & access.

## A five-minute tour

1. **Home** (`/`): scroll the hero (a CSS scroll-driven story), pick dates, then **See prices for these dates**.
2. **A room page**: a nightly price calendar, the rate plans with GST, and **Reserve**, which holds the room for 10 minutes.
3. **Checkout and payment**: a guest signs in by emailed code; the payment page is a test gateway (UPI QR, card, net banking, wallet), and every option ends in a signed test payment. Nothing typed leaves the browser.
4. **Enquire** (`/enquire`): send a stay, wedding or offsite question. It appears in the staff inbox within seconds, classified, routed and drafted.
5. **Staff console**: Today (what needs a reply now, the week's arrivals and departures), the Inbox (open an enquiry, edit the draft, Send), Rates & allotment (the heat map, bulk edits), Analytics.

Nothing in the console sends anything on its own: replies go out only when a person presses Send. Mail to reserved test domains (`@example.test` and the like, which the seed data uses) is recorded as simulated, never delivered.

## Where it runs

| Part | Where |
|---|---|
| Web (Next.js 16) | Vercel, `kaapi-estate.vercel.app`; `/api/*` is proxied to the API |
| API and background jobs (Go) | Railway, from `apps/api/Dockerfile`; the jobs (AI queue, Gmail, outbox, reminders, retention, exchange rates) run in the API process |
| Database | Supabase Postgres (Mumbai), session pooler; migrations run when the API starts; row-level security on every table |
| Email | One connected Gmail mailbox (OAuth, read + send scopes only) |
| AI | Gemini models round-robin, Groq as failover, keyword rules as the last fallback |

Deploying is a push to `main`: Vercel and Railway build from it. New migrations apply on the API's next start.

## Running it locally

Needs Go 1.27 and Node 22+.

```bash
# API
cd apps/api
cp ../../.env.example .env        # fill in the values (names only are in git)
go run ./cmd/devdb                # local Postgres 17 on :54329; leave it running
go run ./cmd/seed -reset          # migrate and load the demo data
go run ./cmd/api                  # :8080

# Web
cd apps/web
npm install
npm run dev                       # :3000, proxies /api to API_ORIGIN (default http://localhost:8080)
```

Tests: with `devdb` running, `TEST_DATABASE_URL=postgres://kaapi:kaapi@localhost:54329/kaapi_test?sslmode=disable go test ./... -p 1` in `apps/api`; `npx tsc --noEmit && npx eslint src --max-warnings=0 && npx next build` in `apps/web`.

## Settings

Every setting is an environment variable. The names are in `.env.example`; values are never committed. The ones that matter most:

- `DATABASE_URL`: Postgres. On Supabase from a host without IPv6, use the session pooler URL.
- `JWE_SECRET` (session tokens), `PII_KEY` (encrypts guests' personal data; **losing it loses that data**), `MOCK_PAYMENT_WEBHOOK_SECRET`: 32+ random bytes each.
- `APP_ORIGIN`: the site's address (CORS, cookies, links in emails).
- `GEMINI_API_KEY`, `GROQ_API_KEY`: optional; without them the keyword rules route enquiries.
- `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GMAIL_REFRESH_TOKEN`: the connected mailbox (`go run ./cmd/gmail-auth` makes the token once).
- `STAFF_PASSWORD_ONLY`: staff emails that skip the emailed code (the reviewer account); empty turns it off.
- `TRUSTED_PROXY_HOPS`: how many proxies sit in front of the API (Vercel then Railway: 2), so client IPs can't be forged.
- Web: `API_ORIGIN` (where `/api/*` goes), `NEXT_PUBLIC_APP_URL`.
