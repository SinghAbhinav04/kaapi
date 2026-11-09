# Code review, 9 Nov 2026

Three local review passes (ultrareview caps a run at 8,000 changed lines; the codebase is ~59,000), then
a browser and performance pass. Status: **fixed**, **won't fix** (with the reason) or **open**.

## Pass 1: change capture, caches, live consoles (since `2d749c9`)

| # | Where | Finding | Status |
|---|---|---|---|
| 1.1 | analytics/service.go | Invalidation during a computation is lost: the stale report is cached afterwards | fixed: generation counter; a report computed across an invalidation isn't stored |
| 1.2 | web apis/changes/hooks.ts | After the SSE stream reconnects nothing refetches what changed in the gap | fixed: `useEventStream` refetches everything it follows on every reopen |
| 1.3 | web apis/changes/hooks.ts | EventSource can't refresh the 15-min token: after expiry the stream 401s forever | fixed: the session is renewed before reconnecting a closed stream |
| 1.4 | migrations/0013 | Triggers notify on zero-row statements | fixed earlier (0014) |
| 1.5 | platform/changes | Full subscriber buffer drops topics | fixed: overflow swaps the oldest topic for `*` |
| 1.6 | httpx/servertiming.go | Query count on sign-in routes is an account-enumeration oracle | fixed: `/v1/auth/*` carries no Server-Timing (contract updated) |
| 1.7 | cmd/api | Change feed pins a pooled connection for life (+ inbox hub's) | open: set `pool_max_conns` in DATABASE_URL on deploy; merging the two listeners is the real fix |
| 1.8 | platform/changes | Connection returned to the pool still LISTENing | open (only on shutdown or an error path) |
| 1.9 | app/app.go | `inventory` topic (holds, price edits) clears analytics | won't fix: analytics reads `night_inventory` allotment, so the topic is needed; splitting holds out needs a migration and buys little at one resort's traffic |
| 1.10 | analytics | Room type / plan renames don't invalidate the cache | won't fix: the 10-min TTL covers a rare rename |
| 1.11 | web rates/optimistic.ts | Optimistic stop-sell keeps an old reason the server clears | fixed |
| 1.12 | web rates/hooks.ts | A rate save refetches every grid twice (onSettled + stream) | won't fix: onSettled is the reconcile when the stream is down |
| 1.13 | changes.go / stream.go | Two copies of the LISTEN loop | open (refactor) |
| 1.14 | stream.go | changesStream duplicates inboxStream's SSE plumbing | open (refactor) |
| 1.15 | web | useDataChanges duplicates useInboxStream | fixed: both use `apis/http` `useEventStream` |
| 1.16 | servertiming.go | Responses that write nothing get no Server-Timing | open (low) |

## Pass 2: booking engine, pricing, AI, enquiries, gateway, offers

| # | Where | Finding | Status |
|---|---|---|---|
| 2.1 | booking/service.go | REFUND_PENDING orders reprocessed by later events (decline wipes the refund marker; capture re-alerts and re-emails) | fixed: `PaymentStatus.Settled()`; test added |
| 2.2 | ai/pipeline.go | Keyword *secondary* COMPLAINT ignored when the model ran (rule 5) | fixed: `sawComplaint` on both classifiers |
| 2.3 | ai/redact.go | `\b` is ASCII-only: non-ASCII names reach the model (rule 7) | fixed: Unicode word edges; test with José, Devanagari, Kannada |
| 2.4 | enquiry/service.go | Retry-AI undoes a staff re-route | fixed: a HUMAN routing keeps its lane, owner and priority |
| 2.5 | enquiry/inbox.go | Re-route with a category change buries a staff-written draft | fixed: `keepsLatestDraft` shared with the worker |
| 2.6 | inventory/hold.go | Releasing a hold with a booking awaiting payment sends the payment to refund | fixed: such a hold isn't released (it runs out by itself) |
| 2.7 | inventory/availability.go | Search and alternatives ignore closed-to-departure | fixed (one more query per search: seven) |
| 2.8 | enquiry/draft.go | "villa" in the message hides rooms; villas full → "fully booked" | fixed: villas first, then rooms |
| 2.9 | enquiry/service.go | Comment claims SKIP LOCKED; a pre-transaction failure blocks the queue | fixed: honest comment; every failure parks the enquiry as FAILED |
| 2.10 | ai/pipeline.go | Comment says FALLBACK is retried later; nothing does | fixed (comment): staff retry from the inbox |
| 2.11 | booking/store.go | Concurrent holds past a promo cap all confirm; used_count stops at the cap | won't fix: deliberate, the guest was shown that price; the DB CHECK keeps the count ≤ cap |
| 2.12 | enquiry/draft.go | Plans[0] labelled "room only" even when it isn't | fixed: plans found by code |
| 2.13 | enquiry/draft.go | Offer percent truncated (12.5% → 12%) | fixed |
| 2.14 | enquiry/service.go | Spam threads still raise GUEST_REPLY alerts | fixed |
| 2.15 | booking/service.go | Refund email hard-codes "10-minute hold" | fixed |
| 2.16 | enquiry/inbox.go | Inbox search doesn't escape `%` `_` | fixed |
| 2.17 | gateway.go | Cookie write with no Origin passes CSRF; native origins on staff writes | won't fix: browsers always send Origin on cross-site writes, and a stolen cookie can set any Origin; cookies are SameSite Lax/Strict |
| 2.18 | enquiry/service.go | Ref allocation scans the whole table | open (fine at 12 months of one resort's mail) |
| 2.19 | reminders.go / enquiry | Resort "today" implemented three times | fixed: `domain.ResortToday` (inventory keeps its settings-based one) |
| 2.20 | enquiry/types.go | Raw strings instead of enums | fixed: draft-author constants, `domain.PromoType` |
| 2.21 | enquiry/service.go | Magic numbers (5000, '1 hour') | fixed |
| 2.22 | enquiry/draft.go | Villa detected by slug substring | fixed: `room_types.kind` carried through search |
| 2.23 | enquiry/service.go | Quote search runs inside the processing transaction | open (refactor) |

## Pass 3: auth, outbox, Gmail, retention, campaigns, access, fx, frontend

| # | Where | Finding | Status |
|---|---|---|---|
| 3.1 | gmail/service.go | Over 50 pending messages: the poll re-reads the same 50 forever | fixed: every message is handled before the position moves |
| 3.2 | outbox/service.go | Staff messages (and replies) to reserved test domains go through Gmail | fixed: `domain.ReservedAddress` on every send path; test added |
| 3.3 | web http/const.ts | A wrong staff code refreshes and resends: each typo burns 2 of 5 tries | fixed |
| 3.4 | access/service.go | users:manage could grant the Administrator role, reset an admin's password, or grant permissions it lacks | fixed: admin-only for the admin role and accounts; no granting beyond your own; test added |
| 3.5 | web PaymentMethods.tsx | UPI test auto-pay re-fires after a failure and overrides a decline | fixed: fires once; any other attempt stops it |
| 3.6 | notify/store.go | An admin's "mark all read" marked every team's alerts read | fixed: admins count and clear only alerts addressed to them |
| 3.7 | gmail/service.go | Poll holds the token mutex across network calls; Send waits | fixed: separate poll lock |
| 3.8 | access/service.go | Changing a role's team doesn't revoke members' sessions | fixed |
| 3.9 | web checkout/hooks.ts | After booking, a flash of "This booking isn't open in this tab" | fixed: ticket cleared on unmount |
| 3.10 | web StaffLoginPage.tsx | Password-only login re-enables the button before navigation | fixed |
| 3.11 | campaign/service.go | Automatic offers announced as "use code X" | fixed |
| 3.12 | web AlertsBell.module.css | Alerts panel overflows at 390 px | fixed: fixed-position panel on phones |
| 3.13 | web EnquiryDetail.tsx | Follow-ups keyed by received_at | fixed |
| 3.14 | gmail/service.go | Skipped messages counted as imported | fixed: `Deliver` reports whether it stored |
| 3.15 | web confirmation/lib.ts | Blob URL revoked before the download starts | fixed |
| 3.16 | web team/lib.ts | Modulo bias in generated passwords | fixed: rejection sampling |
| 3.17 | auth/service.go | Refresh reuse clears every user's session cache | won't fix: rare, and only costs a cache miss |
| 3.18 | notify/service.go | Two queries per bell refresh | open (low) |
| 3.19 | outbox + gmail | `truncate()` duplicated | open (low) |
| 3.20 | auth/service.go | Email normalised twice | open (low) |
| 3.21 | fx/service.go | SQL in service.go (no store.go) | open (refactor) |
| 3.22 | access, campaign | Magic numbers (40, 3, 10) | open (low) |
| 3.23 | web inbox/const.ts | Doc comment detached from QUOTE_LINE | fixed |
| 3.24 | web StaffLoginPage.tsx | Copy inline in JSX | fixed: `staff-login/const.ts` |

## Browser and performance pass

Claude in Chrome wasn't connected, so this used headless Chrome (Lighthouse 12.2.1, and DevTools-protocol
screenshots at 390 and 1440 px, signed in as the local admin).

| # | Finding | Status |
|---|---|---|
| 4.1 | **Vercel serves every photo at full size.** `/_next/image` returns the original AVIF at every width (394 KB at w=390), even to a browser that only accepts WebP: AVIF sources pass through (and Hobby has a transformation quota). Phones downloaded 2400 px photos | fixed: `build-images.py` writes 480/960/1600 copies; a custom loader (`shared/lib/image-loader.ts`) picks one; `/images/*` cached 30 days. Home on mobile: Performance 75 → 85, LCP 5.7 → 4.3 s, weight 983 → 548 KB (local production build) |
| 4.2 | The mid-scroll coffee-rows photo was fetched at high priority, competing with the first photo | fixed: `fetchPriority="low"` |
| 4.3 | `/favicon.ico` 404 on every page (console error, Best Practices 96) | fixed: `app/icon.svg` |
| 4.4 | Bookings status filter ran off the screen at 390 px | fixed in `Segmented` (scrolls inside itself) |
| 4.5 | Dashboard looked empty: weekly numbers only, mostly zero | fixed: now **Today**: replies owed past 30 min (urgent first), complaints, refunds to make, tonight's occupancy, the next 7 days of arrivals and departures, and the week's goals |
| 4.6 | Team and Offers measure wider than 390 px under Chrome's mobile emulation, though every element sits inside the screen (tables scroll in their own boxes) | open: check on a real phone |
| 4.7 | First cold request to the home page took ~4 s to first paint (warm: 1.3 s) | open: Vercel cold start; keep the site warm before reviewers look |

Other scores (live site): privacy page mobile 98, home desktop 95, Accessibility 100 on home.
