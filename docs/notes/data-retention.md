# Data retention and deletion (notes, not built yet)

Status: **the scheduled sweep is built** (`apps/api/internal/retention`, see AGENTS.md § How retention works). Rules decided by Abhinav: finished bookings are archived, contact details wiped 12 months after check-out, financial records kept 8 financial years, then deleted. Items 2 and 4 below (guest delete-my-data requests, staff queue and export) are still to do.
The privacy page (`apps/web/src/app/(guest)/privacy/page.tsx`) already promises parts of it,
so the TDD must label those lines as policy plus a next step until this ships.

## What the privacy page currently says

- Booking records are kept as long as tax law requires.
- Enquiries that never become bookings are deleted after 12 months.
- Guests can ask to see, correct or delete their data, or withdraw consent; we reply within 30 days.

## What will need building (once the rules are decided)

1. **Scheduled purge.** A periodic job in the API, like `SweepBookings`. It runs `FOR UPDATE SKIP LOCKED`
   in batches, so it never blocks the inbox. It finds enquiries past the retention window with no
   `linked_booking_id` and deletes or anonymises them. `enquiry_events`, `enquiry_drafts` and
   `notifications` cascade. `outbox_messages` holds copies of replies (`ON DELETE SET NULL`), so it
   needs its own rule.
2. **Delete-my-data request.** A guest is verified by email OTP (the same flow as My bookings),
   then a request is recorded for staff to action within 30 days. Every step is written to `audit_logs`.
3. **Bookings.** These probably can't be hard-deleted inside the tax window, so the likely approach is
   to anonymise the guest fields and keep the money and dates. Needs the CA's answer.
4. **Staff UI.** A queue of deletion requests, with an "export this guest's data" button (DPDP right to access).
5. **Tests.** Prove the purge leaves bookings, inventory and `sold` counts untouched.
   `TestEveryTableHasRowLevelSecurity` already guards any new table.

## Open questions for the logic

- Retention window per record type: enquiries, abandoned holds, failed payments, OTP codes, sessions.
- Delete or anonymise? Which fields count as personal (name, email, phone, message text, IP hashes)?
- Does the 30-day clock start at the request or after OTP verification?
- Who may approve a deletion (GM only, or admin)?
- What happens to an enquiry that later turns into a booking?
