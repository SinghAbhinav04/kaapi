# AGENTS.md — working notes for anyone (human or AI) picking up this repo

Read this file before changing anything. It is the single source for **what the product is,
how the code is organised, the rules the code follows, and what is left to do**. If you
change a convention, update this file in the same commit.

---

## 1. The product

**Kaapi Estate** is a direct-booking website and an AI-routed enquiry inbox for a
76-key coffee-estate resort in Chikkamagaluru, Karnataka (64 rooms + 12 villas). It is a
take-home for the Isocode SWE-intern assessment, **Brief 4**, due **9 Nov 2026**.

"Kaapi Estate" is a working name. The brief only says "Company A". Every assumption must be
labelled in the TDD, and only facts from the problem statement may be used.

### The client's problem (from the brief)
- About 68% of room nights come through agencies at about 20% commission, so direct bookings are the goal.
- Enquiries arrive scattered across the web form, email, WhatsApp and the event form, and replies are slow.
- **The hard part: never overbook.** Agencies hold the rest of the 76 keys; the website sells only its allotment.

### What is built (core slice)
| Area | What it does |
|---|---|
| Guest site | Home with a scroll-story hero, room pages with price calendar and GST per night, 3-step booking (room → details → payment), mock payment gateway, confirmation, "My bookings" via email one-time code, enquiry/event form, privacy page |
| Booking engine | 10-minute holds that lock inventory, idempotent bookings, HMAC-signed payment webhooks, late-payment refund path, background expiry |
| Staff console | Inbox with AI routing and staff-written replies, "New message" compose, rates and allotment heat map with bulk edits, bookings list (current / archived) and daily CSV export, outbox, dashboard, analytics (GM and admin) |
| AI | Classify → extract → route → draft. Gemini round-robin across every allowed model, Groq failover, keyword rules as the final fallback and complaint safety net |
| Gmail | Admin connects a mailbox once (OAuth). New inbox mail is polled into enquiries; replies and new messages go out from it, threaded. Unconnected, everything stays simulated in the outbox |
| Retention | Job archives finished bookings, wipes guest contact details at 12 months, deletes bookings after 8 financial years and messages at 12 months |
| Analytics | Revenue, ADR, commission saved, cancellations, booking pace, lead time, on-the-books occupancy, enquiry mix, currency guests read prices in |
| Currency | Guests see "≈ $497" beside INR prices in their own currency (country from Vercel's header, else a time-zone guess, else INR; a switcher overrides). Always charged in INR; the currency shown and that day's rate are kept on the booking |

### Rules from the brief that must never break
1. **Never overbook.** Enforced in three layers: row locks, live hold counting, and a DB `CHECK (sold <= allotment)`.
2. **Prices come from the rate table, never the AI.** The server fills `{{QUOTE}}`, and any money the model writes is stripped and flagged.
3. **Drafts are never auto-sent.** Only `POST /v1/staff/enquiries/{ref}/send`, pressed by a person, sends anything.
4. **Low-confidence enquiries go to the Unsorted lane** (the GM sees it). **Re-routing is one click.**
5. **Complaints always alert the GM.** If either the model or the keyword rules sees a complaint, the GM is alerted.
6. **GST shown at the tariff's rate:** per night, 5% up to ₹7,500, 18% above, on the price after discount. This is a labelled assumption.
7. **DPDP:** consent at checkout, minimal fields, no passport details online, personal data redacted before any AI call, and the retention rules in `internal/retention` (the privacy page states the same).
8. **The booking flow is 3 steps or fewer.** It must be fast on slow mobile networks.

---

## 2. Repository layout

```
/
├─ AGENTS.md              ← you are here (CLAUDE.md imports it)
├─ contract/openapi.yaml  ← the API contract (hand-written, enforced by tests)
├─ apps/api/              ← Go backend
├─ apps/web/              ← Next.js frontend (+ Capacitor shell, planned)
├─ docs/                  ← ai-eval-report.md, notes/, TDD + deployment.md (planned)
├─ infra/                 ← Dockerfile / deploy config (planned)
└─ .env.example           ← variable NAMES only, never values
```

---

## 3. Backend: `apps/api` (Go 1.27, module `kaapi/api`)

**Stack:**
- chi v5 router; pgx v5 / pgxpool (hand-written SQL, no ORM, no sqlc); goose migrations embedded in the binary
- go-jose v4 (JWE tokens); argon2id (staff passwords)
- kin-openapi, in tests only: validates responses against the contract
- fergusstrange/embedded-postgres, for local tests only

```
cmd/
  api/      HTTP server: config → migrate → serve; booking sweep + enquiry worker goroutines
  seed/     migrate + load seed/data/*.json (flags: -reset)
  eval/     AI evaluation over the 60 labelled enquiries → docs/ai-eval-report.md
  devdb/    throwaway embedded Postgres 17 on :54329 (+ kaapi_test db) — tests only
internal/
  domain/     shared types and typed-string enums (mirror every DB CHECK constraint)
  platform/   building blocks, no business logic:
    apperr/   error Codes, Code→HTTP status table, Error type, From() classifier
    httpx/    request id, recover, error envelope writer, strict JSON decode, params
    config/   env loading (+ LoadDotEnv for local .env), validation of every var
    db/       Connect, Migrate, WithTx (retries serialization/deadlock errors)
    idgen/    UUIDv7, booking refs (KS-XXXXXX)
    money/    paise formatting with Indian digit grouping
  pricing/    PURE: nightly price, plan derivation, GST slabs, promos (no I/O)
  inventory/  availability search, HOLD ENGINE (the overbooking guard), calendar
  catalog/    rooms, photos, plans, property facts
  booking/    create from hold, payment events, sweep, queries, CSV export
  payment/    provider boundary: HMAC sign/verify, mock events
  auth/       staff login, guest OTP, JWE sealer, refresh rotation, permissions
  admin/      rates/allotment grid + bulk edits (audit-logged)
  enquiry/    intake, worker, routing rules, drafts, inbox ops, stats
  ai/         redaction, keyword classifier, Gemini + Groq clients, model chain, prompts
  outbox/     every sent message (simulated or delivered) + staff-composed messages
  gmail/      OAuth connect, encrypted token store, inbox polling, threaded sending (plain net/http)
  retention/  archive / anonymise / purge sweep (batched, SKIP LOCKED, audit-logged)
  analytics/  read-only revenue, pace, mix and on-the-books report
  fx/         ECB rates via Frankfurter, stored as INR-per-unit micros; shown only while fresh (48 h)
  api/        HTTP layer: router (gateway), handlers, DTOs (types.go), mappers
  seed/       fixture loader + seeder (computes derived values, checks invariants)
migrations/   0001_init.sql (24 tables), 0002_lock_down_data_api.sql (RLS on every table),
              0003_retention.sql (bookings.archived_at / anonymised_at), 0004_gmail.sql (integrations table,
              enquiry external ids, draft send claim, outbox status + STAFF_MESSAGE), 0005_fx.sql (fx_rates,
              bookings.display_currency + rate snapshot). New tables must ENABLE RLS.
seed/data/    JSON fixtures: property, room_types, rate_plans, rates_rules, promos,
              users, enquiries (60, labelled), bookings (40)
tests/
  apitest/    starts the real router on a seeded DB; validates EVERY response against openapi.yaml
  testdb/     migrate + reseed (refuses Supabase URLs — tests truncate tables)
  unit/<pkg>/ and integration/<area>/
```

### Backend conventions (follow these)
- **Files per package:** `service.go` (the package's main entry point, with constructor and methods), `types.go` (structs), `errors.go` (sentinel and typed errors), plus one file per concern (`hold.go`, `quote.go`, …). No barrel files. Interfaces are defined where they are consumed.
- **Shared types live in `internal/domain`** (`Stay`, `Party`, every enum). Never pass raw state strings; use the typed enums, which have `Valid()`.
- **Imports** are short module paths: `kaapi/api/internal/...`.
- **Errors:** domain errors implement `AppError() *apperr.Error`, or are `apperr.New(code, msg)` sentinels. Handlers just `return err`, and one place (`httpx.Handle` → `WriteError`) maps every error to the envelope `{error:{code,message,details,request_id}}`. Messages are written for people. Causes are never leaked. Validation details go in `details.fields` with keys like `guest.email`.
- **Money is `int64` paise.** Never floats.
- **Dates:** stay dates are `time.Time` at midnight UTC, representing IST calendar dates. On the wire they are `YYYY-MM-DD`.
- **IDs** are UUIDv7. Booking refs match `^KS-[2-9A-HJ-NP-Z]{6}$`.
- **No magic numbers.** Name constants next to their use (`const ( … )` in types.go).
- **Transactions** go through `db.WithTx`. **Never hold a transaction open across a network call:** the AI worker reads, calls the model with no transaction open, then re-locks and re-checks before writing. Sending works the same way: claim (`enquiry_drafts.sending_at`) → deliver → record in a second transaction; a failed delivery releases the claim.
- **Comments** explain *why*, and stay short. Match the existing tone.
- **Contract-first:** add or change an endpoint in `contract/openapi.yaml` first. `tests/apitest` fails if a real response doesn't match, which catches drift. Then regenerate the web types (`npm run gen:api` in apps/web).
- **Permissions** are a closed enum in `auth/permissions.go`, granted per role in one table. Routes declare theirs with `requirePermission`. Lane scoping (which inbox lanes a role sees) lives in `api/enquiries.go`.

### How "never overbook" works (`inventory/hold.go`)
1. `SELECT … FROM night_inventory … ORDER BY date FOR UPDATE`. Locking in date order means no deadlocks.
2. Check stop-sell, min-stay, closed-to-arrival and closed-to-departure.
3. Count live holds (`status='ACTIVE' AND expires_at > now()`). Expired holds stop counting with no cron job.
4. Require `sold + held + rooms <= allotment` for every night, then insert the hold with a frozen quote and a hashed token.
5. Confirming (on a verified payment webhook) re-locks, increments `sold`, writes `booking_nights`, and marks the booking `CONFIRMED`. A webhook that arrives late with no capacity left takes the `REFUND_PENDING` path and alerts the GM.
6. Last line of defence: `CHECK (sold <= allotment)` in the database.

Tests prove it: 25 goroutines race for the last pool villa and exactly 1 wins. Overlapping stays never oversell.

### How the AI works (`ai/` + `enquiry/`)
1. **Intake** from any channel stores the enquiry with `ai_status=PENDING`. Deduplicated per sender per day; at most 5 per sender per hour.
2. **The worker** (`enquiry.Service.Run`) wakes on intake or on a 15 s tick and takes the oldest pending enquiry.
3. **`ai.Redact`** removes emails, phone numbers (10+ digits) and the sender's name before any model sees the text.
4. **`ai.Chain`** works through three tiers of models:
   - Tier 1: fast Gemini models, round-robin.
   - Tier 2: Gemini Pro.
   - Tier 3: Groq (gpt-oss, Qwen).

   Each model is paced to its RPM (`GEMINI_RPM`, `GROQ_RPM`). A failing model rests: for 429s as long as Google's RetryInfo says, 30 s after a 5xx, 1 h after a 4xx. The model list is discovered in the background at start-up (`ai.Start` then `Refresh`).
5. **The keyword classifier** (`ai/keyword.go`, English, Hinglish and Kannada-English) always runs. It is the fallback when every model fails (`ai_status=FALLBACK`) and the complaint safety net.
6. **Routing** is a rule table (`enquiry.RouteWith`):
   - complaint seen by either classifier → GM lane, priority HIGH, plus an alert
   - confidence < 0.6 → Unsorted
   - ROOM → Reservations
   - WEDDING or OFFSITE → Sales
7. **Drafts:** the model writes wording only, using the placeholders `{{GUEST_NAME}} {{QUOTE}} {{EVENT_TEAM_CONTACT}} {{SIGNATURE}}`. `ai.Guard` deletes any line where the model wrote money. The server fills `{{QUOTE}}` from `inventory.Search`, which gives the same numbers as the website.
8. **Evaluation:** `go run ./cmd/eval` (with keys set) writes `docs/ai-eval-report.md`. The live result was 59/60 correct, 0 sent to the wrong team, and 11/11 complaints reached the GM. A keyword-based confidence blend was *removed* because the evaluation showed it changed nothing. Keep decisions data-driven like this.

### How retention works (`retention/`)
Runs at start-up and every 6 h (`SweepRetention`), and on demand via `POST /v1/staff/retention/run` (admin). Every step is idempotent.
1. **Archive:** CONFIRMED / CANCELLED / REFUNDED bookings get `archived_at` once `check_out` has passed. Archived rows leave the default staff list (`?archived=true` shows them) but keep nights, payments and audit links. No separate archive table.
2. **Anonymise:** 12 months after check-out, email → `erased+<ref>@example.invalid`, phone/notes/arrival window/guest account cleared. Name, dates, money and GST stay (the tax record).
3. **Purge:** archived bookings are deleted once 8 full financial years (Apr–Mar) have passed after the stay's FY (`PurgeBefore`; Companies Act s.128 outlasts GST's 72 months). Labelled assumption.
4. Also: all enquiries, outbox copies and notifications at 12 months; unpaid bookings at 90 days; dead holds at 7 days; OTPs after a day; sessions 30 days after expiry/revocation. Inventory `sold` is never touched.
Each run writes one `audit_logs` row with the counts.

### How Gmail works (`gmail/`)
- `POST /v1/staff/integrations/gmail/connect` (GM/admin, `integrations:manage`) returns Google's consent URL with an HMAC-signed state. Google redirects to `GET /v1/integrations/gmail/callback`, which stores the refresh token AES-GCM-encrypted (key derived from `JWE_SECRET`; rotating that secret means reconnecting) and redirects to `/staff/inbox?gmail=connected`.
- Scopes: `gmail.readonly` + `gmail.send` only. In Google Cloud the redirect URI must be exactly `APP_ORIGIN/api/v1/integrations/gmail/callback`, and while the consent screen is in Testing mode the mailbox must be a test user.
- A poller (`GMAIL_POLL_SECONDS`, default 30) reads the history API from the stored `history_id` (falls back to the last day on a 404), skips SENT/DRAFT/SPAM and our own address, strips quoted history, and calls `enquiry.Intake` with the Gmail ids. `enquiries.external_id` is unique, so re-polling is harmless; imported mail skips the 5-per-hour sender cap.
- `enquiry.Send` and `outbox.Send` take a small `Mailer` interface (defined in each consumer); with it connected, email replies go out with `In-Reply-To`/`threadId` and the outbox row is `SENT`, otherwise `SIMULATED`. WhatsApp stays simulated.
- Tests use a fake Google server (`tests/integration/api/gmail_test.go`); `GOOGLE_OAUTH_BASE_URL` / `GMAIL_API_BASE_URL` point at it.

### How currencies work (`fx/` + `features/currency`)
- **Prices and charges are INR, always.** The quote, payment, GST and invoice never change; another currency is only an estimate printed beside the INR figure, marked "≈" with "you're charged in INR".
- Rates: `RefreshFX` fetches the ECB rates from Frankfurter at start-up and every 6 h (EUR base, crossed to INR with exact rationals), stored as `inr_per_unit_micros` (INR per unit × 1,000,000, no floats). `GET /v1/fx-rates` returns only rates fetched in the last 48 h, so a broken feed shows INR only. ECB publishes no AED. A `-reset` keeps fetched rates (like the Gmail connection).
- Which currency: the `kaapi_ccy` cookie. `src/proxy.ts` sets it on a first visit from `x-vercel-ip-country` (`features/currency/geo.ts` maps country → currency, India and unlisted countries → INR). Without the header the page guesses from the browser's time zone. The header and room-page switcher override both. The server renders INR only; the estimate appears after hydration (`useSyncExternalStore`), so there's no mismatch.
- On booking, `display_currency` is sent and the server snapshots *its own* fresh rate (`fx.Snapshot`), never one from the client. Analytics groups confirmed bookings by it (`by_display_currency`; none recorded = INR).

### Auth model
- **Access token:** JWE (dir + A256GCM), 15 minutes. Claims hold only user, role and session.
- **Refresh token:** random, stored hashed, rotated on every use. Reusing an old one revokes the whole session chain.
- **Web clients** get httpOnly cookies (`kaapi_at` and `kaapi_rt` in dev; `__Host-kaapi_at` and `__Secure-kaapi_rt` in production). Writes are CSRF-checked by `Origin`.
- **The refresh cookie's path is the one the browser sees:** `BROWSER_API_PREFIX` (default `/api`, the web proxy) + `/v1/auth`. With the bare `/v1/auth` the browser never sent it and every staff session ended after 15 minutes. `TestRefreshCookiePathMatchesTheBrowserProxy` guards it.
- **Tabs refresh one at a time** (`navigator.locks` in `apis/http/index.ts`): two tabs sending the same refresh token look like theft to the API, which then revokes every session.
- **Native clients** send `X-Client: native` and get tokens in the response body, then use Bearer.
- **Guests** sign in with an email OTP: HMAC-bound, single use, rate-limited. In `DEMO_MODE` the API returns the code, because demo email is simulated.

---

## 4. Frontend: `apps/web` (Next.js 16.4, React 19.3, TypeScript 5.9)

**Stack:**
- App Router, React Compiler on
- TanStack Query v5
- CSS Modules + CSS custom-property tokens (no Tailwind, no UI library)
- next/font (Anton, Instrument Serif, Manrope, Space Mono)
- Motion: CSS scroll-driven animation, no GSAP

```
src/
  app/          ROUTES ONLY — thin pages that prefetch and compose features
    (guest)/    / · rooms/[slug] · book/[holdId] · pay/[orderId] · booking/[ref] · my-bookings · enquire · privacy
    staff/      login · (console)/{inbox,rates,bookings,outbox,dashboard,analytics} behind StaffShell
  apis/         THE ENDPOINT LAYER
    http/       HttpClient interface + fetch implementation, ApiError, headers/consts
    query/      QueryClient factory, Hydrate (server → client cache), Register (error type)
    generated/  schema.ts — generated from contract/openapi.yaml (npm run gen:api)
    <module>/   catalog · booking · payments · auth · enquiries · rates · operations (+ Gmail, compose) · analytics
  features/<x>/ UI per domain (home, room, stay-search, booking-flow, checkout, payment,
                confirmation, my-bookings, enquire, site, staff-shell, staff-login,
                inbox, rates-admin, staff-bookings, operations, analytics)
  shared/       reusable, domain-free: ui/ · lib/ · hooks/ · motion/ · styles/ · const/
```

### The `apis/<module>/` pattern (every module is the same)
| File | Holds |
|---|---|
| `interface.ts` | `export interface XApi { … }`: the contract the app depends on |
| `x.api.ts` | `createXApi(client: HttpClient): XApi` (the implementation) and `export const xApi = createXApi(http)` |
| `types.ts` | re-exported generated schema types, plus params types |
| `const.ts` | `X_PATHS` (endpoint builders), `xKeys` (hierarchical query keys), stale/poll timings |
| `queries.ts` | `xQueries.*` = TanStack `queryOptions(...)`, **shared by server prefetch and client hooks** |
| `hooks.ts` | `'use client'` `useX…` query and mutation hooks (invalidation lives here) |
| `index.ts` | the public surface; import modules only via `@/apis/<module>` |

### The `features/<x>/` pattern
`components/*.tsx` + `*.module.css`, `hooks.ts`, `const.ts`, `types.ts`, `lib.ts` (pure
helpers, easy to test) and `index.ts`. Copy, labels and colour mappings go in `const.ts`, not in JSX.

### `shared/ui/<Component>/`
`Component.tsx` + `Component.module.css` + `types.ts` + `const.ts` (variants) + `index.ts`.
The kit includes Button/ButtonLink, TextField/SelectField/TextAreaField/Checkbox, Pill,
Segmented, Stepper, Skeleton, EmptyState/ErrorState and Logo.

### Layering (enforced by ESLint `no-restricted-imports`)
- `app/` may import features, apis and shared.
- `features/` may import apis, shared, and **other features only through their index** (`@/features/x`).
- `apis/` and `shared/` never import features or app. `shared/` never imports apis.

### Data-fetching rules
- **Server components prefetch** with `makeQueryClient().prefetchQuery(xQueries.y(...))` and wrap the page in `<Hydrate client={qc}>`. The page is server-rendered with real data, and the client doesn't refetch until the data is stale.
- **`placeholderData: keepPreviousData`** on anything paged or filtered (calendar months, availability, inbox, rates grid). The old data stays on screen, dimmed via `isPlaceholderData`, while the new data loads. Never show a blank spinner on a refetch.
- **Polling only while something is settling.** The booking page polls while `PENDING_PAYMENT`; an enquiry's detail polls while `ai_status === 'PENDING'`. The inbox and the rates grid refresh on an interval.
- **Retries** only for retryable failures (network, 5xx, rate limits), never a 4xx. Every error is an `ApiError` (`code`, `status`, `fieldErrors`, `requestId`).
- **Optimistic update** for re-routing (the row leaves the lane at once and rolls back on error).
- **The hold token** lives in this tab's sessionStorage (`booking-flow/storage.ts`) and is sent as `X-Hold-Token`. Read sessionStorage with `useSessionValue`, which avoids hydration mismatches.

### Render discipline
- React Compiler is on. Don't add `useMemo`/`useCallback` unless measured.
- Things that tick write to the DOM through refs: the hold countdown (`useCountdown`) and TrailSpine's scroll drawing. Neither re-renders React.
- Use `useEffectEvent` for callbacks used inside effects.
- Stay search lives in the URL (`?in=&out=&adults=&children=&rooms=`, `features/stay-search`).

### Design system (match the approved canvas: isocodelabs.com style)
- **Tokens** in `shared/styles/tokens.css`:
  - colours: paper `#f5f1ea`, ink `#141210`, estate green `#3f6b4a` (the one accent), cherry `#a5432f` (alerts only)
  - easing: `cubic-bezier(.22,1,.36,1)`
  - durations: .18s / .42s / .9s
- **Type:** Anton for display, with one or two words in Instrument Serif italic (`.accent`). Manrope for body. Space Mono for data (prices, dates, refs, labels).
- **Motion:** only transform and opacity, no bounce, everything off under `prefers-reduced-motion`. The hero is a pinned scroll story in pure CSS (`animation-timeline`). TrailSpine draws a Catmull-Rom line behind the content (z 1 behind content z 2), hidden below 900 px.
- **Scroll performance (don't regress):** nothing may repaint per scroll frame. No SVG `filter` on anything that moves (the hero mist is CSS gradients). TrailSpine paints its path once and reveals it with two counter-translated layers; its dot is a gradient div moved by `transform`. Images that appear mid-scroll load eagerly.
- **Charts** (analytics): hand-rolled HTML/CSS, one series colour `--viz-1: #2f7a4b` (brand green fails the chroma floor as a data colour), hover/focus tooltips, a "Show as table" view on each chart, no dual axes.
- **Navigation:** every guest page can reach every other: the footer is a full sitemap, the room page header is sticky with "All stays", breadcrumbs link back with the stay search kept (`ROUTES.staysWith`).
- **Images:** originals in `apps/web/assets/source` (credits in CREDITS.md). `python scripts/build-images.py` writes AVIF into `public/images`. Room photos must show only that room, with honest alt text.
- **Accessibility:** 44 px tap targets, labelled controls, error text linked with `aria-describedby`, `role="alert"` and `role="status"` used correctly.

### Next 16 gotchas (already handled; don't regress)
- `params` and `searchParams` are Promises.
- `images.qualities` must list every quality used: [60, 75].
- `<html data-scroll-behavior="smooth">`.
- `agentRules: false` (otherwise `next dev` writes its own AGENTS.md here).
- `ref` is a real prop in React 19, so never name a prop `ref`.

---

## 5. Data and environments

- **Shared database: Supabase Postgres (Mumbai, project `mpfjyrmtlkeqryfhmdfy`).**
  - Locally: set `DATABASE_URL` in `apps/api/.env` (git-ignored), using the direct host `db.<ref>.supabase.co:5432`, which is IPv6.
  - Cloud Run has no IPv6, so production must use the **session pooler** URL from Supabase → Connect.
  - If the password contains reserved characters (`@ : / # ? %`), percent-encode them in the URL. An unencoded `@` makes the host parse as `@db.…`.
  - Migrations run on API start and in `cmd/seed`.
  - **RLS is on for every table** (migration 0002). The publishable key exposes nothing. A test fails if any table lacks RLS.
- **Seeding:** `cd apps/api && go run ./cmd/seed -reset`. The demo staff password comes from `SEED_DEMO_PASSWORD`.
- **The real administrator** comes from `SEED_ADMIN_EMAIL` / `SEED_ADMIN_PASSWORD` / `SEED_ADMIN_NAME` (role ADMIN).
  - It is created on every `-reset`, or on its own with `go run ./cmd/seed -admin`, which keeps all other data.
  - Re-running resets its password and revokes its sessions.
  - It is never in the fixtures: a real email and password must stay out of git.
- **Tests never touch Supabase.** They truncate tables. Run `go run ./cmd/devdb`, then `TEST_DATABASE_URL=postgres://kaapi:kaapi@localhost:54329/kaapi_test?sslmode=disable go test ./... -p 1`. `testdb` refuses Supabase URLs.
- **Local Postgres without devdb** (e.g. a network that blocks devdb's binary download): `brew install postgresql@17 && brew services start postgresql@17`, create role `kaapi`/`kaapi` and databases `kaapi` + `kaapi_test`, then use `postgres://kaapi:kaapi@127.0.0.1:5432/...`. Override `DATABASE_URL` on the command line (real env beats `.env`). On corporate Macs `psql` can hang on Kerberos: add `gssencmode=disable` (pgx is unaffected).
- **No IPv6?** The Supabase direct host is unreachable; use the session pooler URL instead.
- **Env names** (values never in git; see `.env.example`):
  - API: `DATABASE_URL`, `BROWSER_API_PREFIX` (optional, default `/api`), `FX_API_BASE_URL` (optional), `JWE_SECRET` (32+ bytes), `MOCK_PAYMENT_WEBHOOK_SECRET` (32+ bytes), `APP_ORIGIN`, `APP_ENV`, `DEMO_MODE`, `SEED_DEMO_PASSWORD`, `SEED_ADMIN_EMAIL`, `SEED_ADMIN_PASSWORD`, `SEED_ADMIN_NAME`, `GEMINI_API_KEY`, `GEMINI_MODELS`, `GEMINI_RPM`, `GROQ_API_KEY`, `GROQ_MODELS`, `GROQ_RPM`, `SENTRY_DSN`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GMAIL_REDIRECT_URL` (optional), `GMAIL_POLL_SECONDS` (optional), `PORT` (default 8080)
  - Web: `API_ORIGIN` (where `/api/*` is proxied; default `http://localhost:8080`), `NEXT_PUBLIC_APP_URL`
- **Demo accounts:**
  - staff: `gm@`, `reservations@`, `reservations2@`, `sales@`, `admin@kaapiestate.test`
  - guest: `guest@example.test` (sign in by OTP)
  - The staff password is `SEED_DEMO_PASSWORD`, given to reviewers in deployment.md and nowhere else.
- **Demo "today"** for the seed data is 2026-11-02 (IST). The default search lands on the first weekend at least 7 days ahead.

### Commands
```bash
# API
cd apps/api
go run ./cmd/seed -reset          # migrate + seed the DB in .env
go run ./cmd/api                  # :8080 (reads .env)
go run ./cmd/eval                 # AI evaluation report (needs GEMINI/GROQ keys)
go vet ./... && gofmt -l internal cmd tests
# Web
cd apps/web
npm run gen:api                   # regenerate src/apis/generated/schema.ts from the contract
npm run dev                       # :3000, proxies /api → API_ORIGIN
npx tsc --noEmit && npx eslint src --max-warnings=0 && npx next build
```

---

## 6. Working rules

- **Commits:**
  - conventional subjects (`feat(api):`, `feat(web):`, `fix`, `test`, `docs`, `chore`, `refactor`)
  - a body that explains *why*
  - authored by **SinghAbhinav04 only**: no `Co-Authored-By`, no tool or AI attribution
- **Secrets:** before every commit, check nothing secret is staged (keys, passwords, connection strings). Only names go in `.env.example`.
- **Assumptions:** label every one, and only use facts from the brief.
- **Definition of done for a change:**
  - API: `go vet` and gofmt clean; unit and integration tests pass; the contract is updated and apitest-validated
  - Web: `tsc`, `eslint --max-warnings=0` and `next build` all pass
- **Go toolchain:** `go.mod` needs Go 1.27.3; with an older Go installed, `GOTOOLCHAIN=auto` downloads it (or unpack it to `~/sdk/go1.27.3` and put its `bin` first on `PATH`).
- **Port 8080 taken** (e.g. ZooKeeper's admin server): run the API with `PORT=8081` and the web with `API_ORIGIN=http://localhost:8081`.
- **Windows dev box:** Git Bash plus PowerShell. LF line endings are enforced by `.gitattributes`. Stopping a `go run`/`npm` wrapper can leave its child process (postgres, node) holding the port, so check `Get-NetTCPConnection` and stop it.

---

## 7. Status and what's next (as of 3 Nov 2026)

**Done:**
- Backend core slice, AI chain with an evaluation, full guest flow and staff console, RLS lock-down.
- Supabase seeded and verified (public key reads nothing).
- Navigation pass (every page reachable) and home scroll performance fix.
- Staff-written replies and follow-ups; "New message" compose.
- Data retention and deletion (`internal/retention`, privacy page updated). Policy from Abhinav: archive, anonymise, keep for the tax window.
- Analytics tab.
- Gmail integration (connect, poll, threaded send). Needs the redirect URI registered in Google Cloud and the mailbox added as a test user.
- Currency estimates for foreign guests, kept per booking for analytics (3 Nov).
- Fixed: web token refresh never worked through the `/api` proxy (refresh cookie path), and parallel tabs could revoke each other's sessions.

**To do, in order:**
1. A visual pass in a browser at 1440 px and 390 px against the approved canvas (Claude Design artifact "Kaapi Estate — Direct Booking").
2. Small API gaps:
   - `GET /v1/staff/notifications`: GM alerts are stored but nothing reads them.
   - Promo admin.
   - A rate limit on staff login.
   - Delete-my-data requests from guests (OTP-verified, staff queue, export), see `docs/notes/data-retention.md`.
   - Inbound replies on an existing Gmail thread become a new enquiry; linking them to the original is a next step.
  - Sessions slide forever: each refresh grants 30 more days, with no absolute cap. Consider one (e.g. 12 h for ADMIN).
3. Capacitor Android shell (`server.url` → production site; Bearer tokens in Preferences).
4. Deploy:
   - API: Dockerfile, Cloud Run `asia-south1`, Supabase session pooler. The retention sweep and Gmail poller are in-process tickers, so either keep `min-instances=1` or have Cloud Scheduler call `POST /v1/staff/retention/run` and `/integrations/gmail/sync`.
   - Production Gmail: add the production redirect URI and publish the consent screen (restricted scope → Google verification).
   - Web: Vercel `bom1`.
   - CI: GitHub Actions running vet, tests against a Postgres service, web lint, typecheck and build.
5. Docs: the TDD as a PDF with the GitHub and live links at the top and every assumption labelled (retention windows, 8 FY, 20% commission estimate), plus `docs/deployment.md` (local run, env names only, demo logins).
