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
| Offers | Festive and seasonal discounts that apply by themselves, plus promo codes; guests always get their best offer. Staff manage them on the console's Offers page |
| Guest email | Sign-in codes, a welcome on first sign-in, booking confirmations, refund notices and stay reminders go out for real through the connected Gmail as branded HTML (queued, retried, never lost). Offer emails only to guests who opted in, with one-click unsubscribe |
| Privacy | Guests', senders' and users' personal data encrypted at rest (AES-256-GCM) with blind indexes for lookups |
| Gateway | CORS and rate limits per API group, live-session token checks, client IPs that can't be forged |
| Currency | Guests see "≈ $497" beside INR prices in their own currency (country from Vercel's header, else a time-zone guess, else INR; a switcher overrides). Always charged in INR; the currency shown and that day's rate are kept on the booking |

### Rules from the brief that must never break
1. **Never overbook.** Enforced in three layers: row locks, live hold counting, and a DB `CHECK (sold <= allotment)`.
2. **Prices come from the rate table, never the AI.** The server fills `{{QUOTE}}`, and any money the model writes is stripped and flagged.
3. **Drafts are never auto-sent.** Only `POST /v1/staff/enquiries/{ref}/send`, pressed by a person, sends anything.
4. **Low-confidence enquiries go to the Unsorted lane** (the GM sees it). **Re-routing is one click.**
5. **Complaints always alert the GM.** If either the model or the keyword rules sees a complaint, the GM is alerted.
6. **GST shown at the tariff's rate:** per night, 5% up to ₹7,500, 18% above, on the price after discount. This is a labelled assumption.
7. **DPDP:** consent at checkout, minimal fields, no passport details online, personal data redacted before any AI call, personal data **encrypted at rest** (`platform/pii`: never store a guest's name, email or phone except through `pii.Text`), and the retention rules in `internal/retention` (the privacy page states the same).
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
  api/      HTTP server: config → migrate → serve (runs the worker jobs too when WORKER_IN_PROCESS, the dev default)
  worker/   background jobs: AI queue, Gmail poll, booking sweep, retention; /healthz on PORT
  gmail-auth/ one-time Gmail consent → saves GMAIL_REFRESH_TOKEN to .env (-save stores an existing connection)
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
  outbox/     every sent message (simulated or delivered) + staff-composed messages; never sends to reserved test domains
  campaign/   offer and festive emails to opted-in guests: watches published offers, fans out, unsubscribes
  gmail/      OAuth connect, encrypted token store, inbox polling, threaded sending (plain net/http)
  retention/  archive / anonymise / purge sweep (batched, SKIP LOCKED, audit-logged)
  analytics/  read-only revenue, pace, mix and on-the-books report
  fx/         ECB rates via Frankfurter, stored as INR-per-unit micros; shown only while fresh (48 h)
  platform/pii/        the personal-data interceptor (pii.Text: encrypt on write, decrypt on read) + blind indexes
  platform/ratelimit/  fixed-window counters: memory per process, or rate_limit_buckets shared by every instance
  platform/changes/    the change feed (LISTEN data_changes): cache invalidation + the staff SSE stream
  platform/cache/      a small TTL cache (the hot store; Redis-replaceable through consumers' interfaces)
  app/        composition root: builds every service once (cmd/api, cmd/worker and tests all use it) + RunWorkers
  transport/httpapi/  HTTP layer: router (gateway), handlers, DTOs (types.go), mappers, SSE inbox stream
  access/     RBAC: roles, grants, staff accounts, cached per-request permission lookup
  seed/       fixture loader + seeder (computes derived values, checks invariants)
migrations/   0001_init.sql (24 tables), 0002_lock_down_data_api.sql (RLS on every table),
              0003_retention.sql (bookings.archived_at / anonymised_at), 0004_gmail.sql (integrations table,
              enquiry external ids, draft send claim, outbox status + STAFF_MESSAGE), 0005_query_indexes.sql,
              0006_inbox_notify.sql, 0007_rbac.sql, 0008_analytics_covering_index.sql, 0009_fx.sql (fx_rates,
              bookings.display_currency + rate snapshot), 0010_outbox_delivery.sql (queued guest emails),
              0011_offers.sql (automatic offers), 0012_pii.sql (blind-index columns for encrypted data),
              0013_data_changes.sql (change-capture triggers),
              0014_data_changes_rows.sql (notify only when rows changed), 0015_reminders_and_staff_codes.sql
              (booking_reminders, STAFF_2FA codes), 0016_marketing.sql (opt-in on users, campaigns,
              campaign_recipients). New tables must ENABLE RLS. Two branches must never
              add the same migration number: goose records versions, so the second 0005 is silently skipped.
seed/data/    JSON fixtures: property, room_types, rate_plans, rates_rules, promos,
              users, enquiries (60, labelled), bookings (40)
tests/
  apitest/    starts the real router on a seeded DB; validates EVERY response against openapi.yaml
  testdb/     migrate + reseed (refuses Supabase URLs — tests truncate tables)
  unit/<pkg>/ and integration/<area>/
```

### Backend conventions (follow these)
- **Files per package:** `service.go` (rules and transaction boundaries, no SQL), `store.go` (every query, as methods taking a `db.Querier` so the service decides pool vs transaction), `types.go`, `errors.go`, plus one file per concern. No barrel files. Interfaces are defined where they are consumed. Done everywhere except enquiry, which still mixes SQL into its service.
- **Queries:** aggregate in Postgres and return totals, never rows to sum in Go. Select only the columns needed (the inbox reads `left(raw_text, 400)`). Any new hot query gets an `EXPLAIN ANALYZE` on production-scale data first; indexes go in migrations as `CREATE INDEX CONCURRENTLY` (`-- +goose NO TRANSACTION`). Avoid `ORDER BY <expression>` on large sets (split into index-ordered halves, as the inbox list does).
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
- Rates: the worker (`app.RunWorkers`) fetches the ECB rates from Frankfurter at start-up and every 6 h (EUR base, crossed to INR with exact rationals), stored as `inr_per_unit_micros` (INR per unit × 1,000,000, no floats). `GET /v1/fx-rates` returns only rates fetched in the last 48 h, so a broken feed shows INR only. ECB publishes no AED. A `-reset` keeps fetched rates (like the Gmail connection).
- Which currency: the `kaapi_ccy` cookie. `src/proxy.ts` sets it on a first visit from `x-vercel-ip-country` (`features/currency/geo.ts` maps country → currency, India and unlisted countries → INR). Without the header the page guesses from the browser's time zone. The header and room-page switcher override both. The server renders INR only; the estimate appears after hydration (`useSyncExternalStore`), so there's no mismatch.
- On booking, `display_currency` is sent and the server snapshots *its own* fresh rate (`fx.Snapshot`), never one from the client. Analytics groups confirmed bookings by it (`by_display_currency`; none recorded = INR).

### Access control (`access/`, migration 0007)
- Tables: `permissions` (catalogue with labels and groups), `roles` (name, description, team, `is_system`), `role_permissions`. `users.role_id` points at the role; a trigger keeps `users.role` (the team) in step. The team still drives lane assignment, GM alerts and signature titles; permissions drive everything a person can see or do.
- Administrator and Guest are system roles (fixed). The Administrator holds every permission in code, including future ones. Built-in GM / Reservations / Sales roles start with the grants the code used to hard-code and are editable.
- Every request checks `access.Service.Can` (30 s cache per process, cleared at once by the process making a change). Deactivating someone cuts access immediately even with a live token; role changes, deactivation and password resets also revoke their sessions. The last active Administrator can't be removed and nobody can lock themselves out.
- Console: `/staff/team` (permission `users:manage`). A test keeps the permissions table and `auth.KnownPermissions()` identical.

### Live inbox
- Migration 0006: a trigger sends the enquiry's ref with `pg_notify('inbox_changes', …)` on every enquiry or draft change, from any process. The API holds one LISTEN connection while any inbox page is open and streams refs as server-sent events (`GET /v1/staff/inbox/stream`); the page refetches. Streams bypass the request timeout and clear the write deadline.
- Gmail is checked every `GMAIL_POLL_SECONDS` (5). "Sync mail" (`POST /v1/staff/integrations/gmail/sync`) catches up on everything since the last successful sync; Gmail message ids are the duplicate guard. Real push (Gmail → Pub/Sub → webhook) needs a public URL and comes with deployment.
- Gmail connects once from the terminal (`go run ./cmd/gmail-auth`) and stays connected via `GMAIL_REFRESH_TOKEN`; there is no connect button in the UI. Google's OAuth client lists only the site root as redirect, so `apps/web/src/proxy.ts` forwards `/?code&state` to the API callback.

### Auth model
- **Access token:** JWE (dir + A256GCM), 15 minutes. Claims hold only user, role and session.
- **Refresh token:** random, stored hashed, rotated on every use. Reusing an old one revokes the whole session chain.
- **Web clients** get httpOnly cookies (`kaapi_at` and `kaapi_rt` in dev; `__Host-kaapi_at` and `__Secure-kaapi_rt` in production). Writes are CSRF-checked by `Origin`.
- **The refresh cookie's path is the one the browser sees:** `BROWSER_API_PREFIX` (default `/api`, the web proxy) + `/v1/auth`. With the bare `/v1/auth` the browser never sent it and every staff session ended after 15 minutes. `TestRefreshCookiePathMatchesTheBrowserProxy` guards it.
- **Tabs refresh one at a time** (`navigator.locks` in `apis/http/index.ts`): two tabs sending the same refresh token look like theft to the API, which then revokes every session.
- **Native clients** send `X-Client: native` and get tokens in the response body, then use Bearer.
- **Guests** sign in with an email OTP: HMAC-bound, single use, rate-limited. **A code is never returned by the API or shown on screen**, only emailed (tests read it from the outbox copy, `emailedCode`). `DEMO_MODE` was removed for this. **Booking requires a signed-in guest** (the email is verified by code at checkout, so a typo can't lose the confirmation).
- **Staff** sign in with two steps: `POST /v1/auth/staff/login` checks the password and emails a 6-digit code (202, masked address only), `POST /v1/auth/staff/verify` signs in. Codes: 5 min, 5 attempts, own purpose (`STAFF_2FA`), so a guest code can't stand in.
- **A token is only as good as its session:** `authenticate` decrypts it, rejects anything over 4 KB, issued in the future or living longer than 15 min, then asks `auth.SessionLive` (session not revoked or expired, user active; cached 30 s, cleared at once in-process on logout and rotation). Logging out or refreshing kills the old access token immediately.

### The gateway (`transport/httpapi/gateway.go` + `policies.go`)
The gateway is a layer in the API process, not a separate service (decision: one Go service plus a worker doesn't earn a second hop, a second deploy, or trusting identity headers between processes; split it out when a second independently deployed service exists). In production, edge protection is managed: Cloud Armor in front of Cloud Run, Cloud Run ingress restricted to the load balancer.
- **Order, for every `/v1` route:** trusted client IP (router, before the access log) → CORS by group → `authenticate` → `rateLimit` by group → `checkOrigin` (CSRF) → the route's `requirePermission`. Routes declare only their permission; CORS and limits live in the `groups` table in `policies.go`, first match wins.
- **Client IP:** `httpx.ClientIP(TRUSTED_PROXY_HOPS)` takes the address that many entries from the right of `X-Forwarded-For` + the connection, so a forged leftmost entry is ignored (chi's `RealIP` trusted it). Default 1 (the web proxy); Vercel + Cloud Run is 2. Correct only while the API is reachable solely through those proxies.
- **CORS by group:** none for the payment webhook and the Gmail return; the web app only for `/v1/staff`; web app + `NATIVE_APP_ORIGINS` elsewhere. Preflights are judged as the method they ask about. `/healthz` and `/readyz` have no CORS.
- **Rate limits (assumptions, sized for one resort):** staff login 10 / 15 min, OTP request 10 / 10 min, OTP verify 20 / 10 min, holds 20 / 10 min (a hold locks a room), bookings 20 / 10 min, enquiries 20 / h: all per IP and counted in `rate_limit_buckets` (shared by every instance). Refresh 60 / min, payment simulator 30 / 10 min, webhook 600 / min per IP; staff 600 / min and everything else 300 / min per account when signed in, else per IP: counted in memory per process. Keys are HMACs (no raw IP or email stored); the retention sweep deletes windows after 24 h. Over the limit: `429 RATE_LIMITED`, `Retry-After`, `details.retry_after_seconds`. A counter outage fails open. `RATE_LIMITS=off` disables them; tests leave them off except `gateway_test.go`.

---

### Guest emails (`outbox/`, migration 0010)
- Sign-in codes, the welcome on a guest's first sign-in (registration), booking confirmations and refund notices are inserted as **QUEUED in the same transaction** as what caused them, so none is lost or sent for something that rolled back.
- The worker's `Outbox.Deliver` runs every 2 s: claim a batch (`SKIP LOCKED`), send through the connected Gmail with no transaction open, then record `SENT` with Gmail's id. Failures retry at 30 s, 1, 2, 4 min and become `FAILED` after 5 tries, with `last_error` kept. A claim older than 2 min is freed (the worker died mid-send). No mailbox connected: `SIMULATED`, as before.
- Staff-written messages and enquiry replies still send straight from the request (a person is waiting); both record what happened in the outbox.

### Reminders (`booking/reminders.go`, migration 0015)
- Four per booking, each **exactly once** (`booking_reminders` primary key): `CHECKIN_SOON` (check-in 1–3 days away), `CHECKIN_TODAY` (from 8 am IST that day), `CHECKOUT_SOON` (two days before check-out, once arrived), `CHECKOUT_TODAY` (7–11 am IST on check-out day). The worker only *checks* every 15 min. Timings are labelled assumptions.

### Offer emails (`campaign/`, migration 0016)
- The checkout's "send me offers" box records `users.marketing_opt_in` and an unsubscribe token (through `booking.Consents`).
- **Observer:** `admin.Service` announces published offers to its `OfferObserver`s inside the offer's transaction; `campaign.Service` is one (wired in `app`). Saving an active offer with `announce: true` creates its campaign, once per offer (unique index), however often it's edited later. Staff also write FESTIVE / ANNOUNCEMENT campaigns (`/v1/staff/campaigns`, `promos:write`).
- `campaign.Dispatch` (worker, every minute) fans each due campaign out into QUEUED outbox rows (`PROMOTION`) in batches of 200; `campaign_recipients` means each guest gets each campaign at most once. Every email ends with a one-click unsubscribe link (`/unsubscribe?token=`, which asks for the click, since mail scanners open links; `POST /v1/marketing/unsubscribe`).
- **Reserved domains:** the outbox records mail to `.test`, `.example`, `.invalid`, `.localhost` and example.com/net/org as SIMULATED, never Gmail (the seed uses `@example.test`; bounces would land in the inbox). Use a real-looking domain in tests that expect delivery through the fake Gmail.
- **The Gmail poller skips bounces and auto-replies** (multipart/report, mailer-daemon/postmaster, `Auto-Submitted`, `X-Autoreply`).

### Payment page (`features/payment`)
- UPI (a real `upi://` QR for a placeholder merchant, or a UPI ID), card, net banking, wallet. With no gateway, each completes the same signed test payment; a shown QR "scans" after 8 s. Nothing typed leaves the browser. `qrcode` 1.5.4 draws the QR.

### Offers (`admin/offers.go`, `inventory/quote.go`, migration 0011)
- An offer is a row in `promo_codes`: percent (basis points, capped at 50%) or flat (paise), stay window, booking window, minimum nights, optional use cap, optional room types. `auto_apply` makes it a festive or seasonal offer that needs no code.
- `quoteStay` prices the stay with the typed code (an invalid one is still reported) and every automatic offer that fits, and keeps the **cheapest total** for the guest; on a tie the typed code wins. The price still comes from the rate table, only reduced.
- Staff with `promos:write` (GM, administrator) use `/v1/staff/offers` and the console's **Offers** page. Every change is audit-logged; nothing is deleted (bookings reference offers), pausing is `active: false`; a use cap can't drop below uses already made.

### Personal data encryption (`platform/pii`, migration 0012)
- **What's encrypted:** bookings `guest_name/email/phone/country`; enquiries `sender_name/email/phone`; users `email/name/phone`; outbox `recipient/body`. Enquiry `raw_text` stays plain on purpose (the AI needs it, it's redacted before any model call, and inbox search uses it): label it in the TDD.
- **The interceptor:** pass `pii.Text(x)` (or `pii.Null(ptr)`) as a query argument and it is encrypted; scan into `pii.Into(&field)` / `pii.IntoNull(&ptr)` and it is decrypted. Domain code and handlers only see plain strings. A new column with personal data must go through these, and the integration test `TestPersonalDataIsCiphertextAtRest` should list it.
- **Scheme:** AES-256-GCM, random nonce (same value encrypts differently each time; tampering fails), stored as `pii:v1:<base64>`; the `v1` leaves room to rotate keys. Encryption and index keys are derived separately from `PII_KEY`.
- **Lookups:** ciphertext can't be compared, so equality goes through keyed **blind indexes** (`pii.EmailIndex`, `pii.Index`; HMAC-SHA256 of the trimmed, lower-cased value): `bookings.guest_email_hash` (My bookings), `users.email_hash` (sign-in, and unique: one account per email), `enquiries.sender_hash` (dedupe and per-sender cap), `otp_codes.email_hash` (codes keep no email at all). Exact match only: no `ILIKE` on encrypted columns; inbox search covers the reference and message, and the team list sorts names in Go.
- **Keys:** `PII_KEY` (32+ bytes) is required in production and lives in `.env.production` / Secret Manager. **Lose it and the data is gone; change it and everything must be re-encrypted.** Development derives one from `JWE_SECRET` (`config.PIIKey`); the seeder and `testdb` derive it the same way, so they always agree.
- **Existing rows:** `seed.EncryptExisting` runs on every API and seed start: it encrypts plaintext rows and fills the indexes in batches, once (a no-op afterwards). Reads pass old plaintext through, so nothing breaks before it runs.

### Change capture, caches and timing (`platform/changes`, `platform/cache`, migration 0013)
- **Change data capture, the light way:** statement-level triggers (migration 0014: they read the statement's transition table and stay silent when it changed no rows, because the minute sweep's UPDATEs usually match nothing) on `bookings`, `night_inventory`, `night_rates`, `holds`, `promo_codes` and `enquiries` send a topic on `pg_notify('data_changes')`: `bookings`, `inventory`, `offers` or `enquiries`, delivered on commit, from any process. `changes.Feed` (run by `cmd/api`) keeps one LISTEN connection and, after any (re)connect, publishes `*` (notices may have been missed). Scale-up path: logical replication or Redis pub/sub behind the same `Subscribe`/`OnChange`.
- **Analytics cache:** `analytics.Service` keeps computed reports (`platform/cache`, in memory, behind the consumer's `reportCache` interface, so Redis can replace it when instances must share). Key: range + today. The feed clears it on `bookings`, `inventory`, `enquiries` or `*`; `ANALYTICS_CACHE_SECONDS` (default 600, 0 = off, the tests' default) is only the safety net. Measured: 13 queries on a miss, 0 on a hit.
- **Never cache what decides a booking:** availability, holds and the hold's price always read the database (overbooking guard, prices from the rate table).
- **Staff consoles:** `GET /v1/staff/changes/stream` (SSE; rates:read, analytics:read, promos:write or enquiries:read) sends each topic; `apis/changes` `useDataChanges` (mounted once in `StaffShell`) invalidates what the topic affects. The rates grid, analytics, the dashboard and the sidebar's counts use `staleTime: Infinity` and don't poll. The sidebar reads `GET /v1/staff/inbox/counts` (lane counts for the caller's lanes, plus the week's median reply when they have dashboard:read) instead of the whole inbox; the inbox and change streams refresh it. Measured on a production build: the rates page makes 5 API calls, each once, and none while idle. In `next dev` you will see requests twice, the first cancelled: that is React Strict Mode mounting twice, not a bug. A rate save applies at once with `setQueryData` (`apis/rates/optimistic.ts`), rolls back on error and reconciles on settle.
- **Guest search batching:** availability loads plans, nightly rates, the typed code and live offers in four queries and prices in memory through `priceStay`, the same pure function the hold uses (`TestSearchQuotesMatchWhatAHoldFreezes` keeps them equal). Six queries per search, down from about fifty.
- **Server-Timing:** every response carries `db;dur=…;desc="N queries", app;dur=…` (a pgx query tracer, `db.Tracer()`, counts per request), so production latency is read in the browser's network panel, not guessed.
- Other per-process caches with short TTLs: permissions (`access`, 30 s), live sessions (`auth.SessionLive`, 30 s), in-memory rate-limit counters. Shared state lives in Postgres: `rate_limit_buckets`, `pg_notify`. There is no Redis yet; add it (Memorystore or Upstash) when several API instances must share the analytics cache and rate limits.

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

- **Two databases.** The local devdb is for `dev` (`apps/api/.env`); Supabase is production, for `main` (`apps/api/.env.production`). See § 6 Branches.
- **Production database: Supabase Postgres (Mumbai, project `mpfjyrmtlkeqryfhmdfy`).**
  - `DATABASE_URL` in `apps/api/.env.production` (git-ignored), using the direct host `db.<ref>.supabase.co:5432`, which is IPv6.
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
  - API (new): `WORKER_IN_PROCESS`, `GMAIL_REFRESH_TOKEN`; `GMAIL_POLL_SECONDS` defaults to 5; `PII_KEY` (32+ bytes, required in production); `DOTENV_FILE` (which settings file the commands load; default `.env`); `ANALYTICS_CACHE_SECONDS` (default 600; 0 = off).
  - API: `DATABASE_URL`, `BROWSER_API_PREFIX` (optional, default `/api`), `FX_API_BASE_URL` (optional), `JWE_SECRET` (32+ bytes), `MOCK_PAYMENT_WEBHOOK_SECRET` (32+ bytes), `APP_ORIGIN`, `APP_ENV`, `SEED_DEMO_PASSWORD`, `SEED_ADMIN_EMAIL`, `SEED_ADMIN_PASSWORD`, `SEED_ADMIN_NAME`, `GEMINI_API_KEY`, `GEMINI_MODELS`, `GEMINI_RPM`, `GROQ_API_KEY`, `GROQ_MODELS`, `GROQ_RPM`, `SENTRY_DSN`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GMAIL_REDIRECT_URL` (optional), `GMAIL_POLL_SECONDS` (optional), `RATE_LIMITS` (optional, default on), `TRUSTED_PROXY_HOPS` (optional, default 1), `PORT` (default 8080)
  - Web: `API_ORIGIN` (where `/api/*` is proxied; default `http://localhost:8080`), `NEXT_PUBLIC_APP_URL`
- **Demo accounts:**
  - staff: `gm@`, `reservations@`, `reservations2@`, `sales@`, `admin@kaapiestate.test`
  - guest: `guest@example.test` (sign in by OTP)
  - The staff password is `SEED_DEMO_PASSWORD`, given to reviewers in deployment.md and nowhere else.
  - These addresses are reserved test domains and can't receive email, so with codes sent by email only they can no longer sign in. Sign in with the real administrator (`SEED_ADMIN_EMAIL`), and give staff accounts real mailboxes (Team page) before reviewers need them.
- **Demo "today"** for the seed data is 2026-11-02 (IST). The default search lands on the first weekend at least 7 days ahead.

### Commands
```bash
# API (dev: the local database)
cd apps/api
go run ./cmd/devdb                # local Postgres 17 on :54329 (kaapi + kaapi_test); leave it running
go run ./cmd/seed -reset          # flush the local DB: migrate + reseed (reads .env)
go run ./cmd/api                  # :8080 (reads .env)
# Production (Supabase): name the file explicitly, every time
DOTENV_FILE=.env.production go run ./cmd/api          # applies migrations + encrypts existing rows
DOTENV_FILE=.env.production go run ./cmd/seed -reset  # WIPES production: only when the owner asks
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

- **Branches (from 3 Nov 2026):**
  - `dev`: every change lands here first. It runs against the **local database** (`apps/api/.env` → devdb on `:54329`). Flush dev data into it with `go run ./cmd/seed -reset`, then test: the Go suite, the web checks, and the app by hand on `localhost:3000`. Push `dev`.
  - `main`: **production** (Supabase, `apps/api/.env.production`; later Cloud Run + Vercel). Only tested work moves here, and only when the owner says so: `git switch main && git merge dev && git push`, then bring production's schema up with `DOTENV_FILE=.env.production go run ./cmd/api` (migrations and the PII backfill run on start).
  - Never point a `-reset` at production unless the owner asks for production to be wiped.
  - Migration numbers are global: check `main` and `dev` before adding one. Goose records versions, so two branches each adding `0005` means one is silently skipped (it happened; fx became 0009).

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

## 7. Status and what's next (as of 4 Nov 2026)

**Done:**
- Backend core slice, AI chain with an evaluation, full guest flow and staff console, RLS lock-down.
- Navigation pass (every page reachable) and home scroll performance fix.
- Staff-written replies and follow-ups; "New message" compose.
- Data retention and deletion (`internal/retention`, privacy page updated). Policy from Abhinav: archive, anonymise, keep for the tax window.
- Analytics tab.
- Gmail integration: always connected (resorts.kaapi@gmail.com locally), 5 s check, catch-up sync, threaded send, drafts that know rooms and offers.
- Production layout: composition root, separate worker, transport layer, repositories for six packages.
- Query audit at production scale with indexes (0005, 0008); live inbox (0006); database-backed RBAC with an admin console (0007).
- Currency estimates for foreign guests, kept per booking for analytics.
- Fixed: web token refresh never worked through the `/api` proxy (refresh cookie path), and parallel tabs could revoke each other's sessions.
- On `dev` (not yet on `main`), 4 Nov: verified-email booking, staff two-step sign-in, branded HTML emails, the four stay reminders, payment options with a UPI QR, offer and festive emails with opt-in and unsubscribe, reserved-domain and bounce guards; the store.go split for gmail, auth, booking and inventory. Earlier on `dev`: gateway policies (CORS and rate limits per group, live-session tokens, trusted client IP); real guest emails through the outbox; automatic offers with the console's Offers page; personal data encrypted at rest; `DOTENV_FILE` for dev/production settings; batched guest search; change capture with the analytics cache and live staff consoles; Server-Timing.

**State of the two databases:**
- Local Postgres (127.0.0.1:5432/kaapi): on `dev`, migrations 1–16, freshly reseeded on 4 Nov, plus one test booking KS-7VT6VT (smoke.guest@example.test, opted in to offers).
- Supabase (production): migrations 1–9 from `main`. 10–16 and the encryption backfill run when `dev` is promoted and the API starts with `.env.production` (whose `PII_KEY` must be kept safe from then on).

**To do, in order:**
1. Promote `dev` to `main` once the owner has tested it (§ 6 Branches), then bring Supabase up to migration 14.
2. After deploying, read real latency from `Server-Timing` and decide whether Redis is needed (several instances).
3. A visual pass in a browser at 1440 px and 390 px against the approved canvas (Claude Design artifact "Kaapi Estate — Direct Booking"), including the currency switcher in the header and the Offers page.
4. Small API gaps:
   - `GET /v1/staff/notifications`: GM alerts are stored but nothing reads them.
   - Delete-my-data requests from guests (OTP-verified, staff queue, export), see `docs/notes/data-retention.md`.
   - Inbound replies on an existing Gmail thread become a new enquiry; linking them to the original is a next step.
   - Sessions slide forever: each refresh grants 30 more days, with no absolute cap. Consider one (e.g. 12 h for ADMIN).
   - Login is limited per IP only; a per-account limit (credential stuffing from many IPs) needs the email, which the gateway doesn't read.
   - Drafts (`enquiry_drafts`) can contain the guest's name and aren't encrypted yet.
   - Finish the store.go split for enquiry (gmail, auth, booking and inventory are done).
5. Capacitor Android shell (`server.url` → production site; Bearer tokens in Preferences).
6. Deploy:
   - API: Dockerfile, Cloud Run `asia-south1`, Supabase session pooler. Workers (retention, Gmail, outbox, exchange rates) run in `cmd/worker` or in-process; keep one instance warm or schedule them. Cloud Armor in front; ingress through the load balancer only (`TRUSTED_PROXY_HOPS` must match).
   - Secrets in Secret Manager, `PII_KEY` above all.
   - Production Gmail: add the production redirect URI and publish the consent screen (restricted scope → Google verification).
   - Web: Vercel `bom1`.
   - CI: GitHub Actions running vet, tests against a Postgres service, web lint, typecheck and build.
7. Docs: the TDD as a PDF with the GitHub and live links at the top and every assumption labelled (retention windows, 8 FY, 20% commission estimate, rate-limit sizes, plaintext enquiry text, the gateway-in-process decision), plus `docs/deployment.md` (local run, env names only, demo logins).
