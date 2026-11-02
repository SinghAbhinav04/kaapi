# Kaapi Estate: direct booking and enquiry routing

This is a take-home build for **Brief 4**: a hill resort's direct-booking website with AI enquiry routing.

> "Kaapi Estate" is a working name for the client the brief calls "Company A" (a 76-key coffee-estate resort in Chikkamagaluru). Every assumption is labelled in the TDD.

## Repository layout

| Path | What it is |
|---|---|
| `apps/web` | Next.js (TypeScript) guest site and staff console |
| `apps/api` | Go API: gateway, inventory/holds, pricing, bookings, payments (mock), enquiries and AI |
| `apps/api/seed/data` | Seed data as language-neutral JSON fixtures. Derived values (nightly rates, sold counts, totals) are computed by the seeder, not stored here |
| `contract` | OpenAPI contract. Both the Go server and the TS client are generated from it |
| `infra` | Local Postgres, Dockerfile, deploy config |
| `docs` | TDD, deployment guide, AI evaluation report, decision records |

Setup and deployment: see `docs/deployment.md` (added as the build progresses).
