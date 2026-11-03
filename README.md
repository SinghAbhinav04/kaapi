# Kaapi Estate: direct booking and enquiry routing

This is a take-home build for **Brief 4**: a hill resort's direct-booking website with AI enquiry routing.

> "Kaapi Estate" is a working name for the client the brief calls "Company A" (a 76-key coffee-estate resort in Chikkamagaluru). Every assumption is labelled in the TDD.

## Repository layout

| Path | What it is |
|---|---|
| `apps/web` | Next.js (TypeScript) guest site and staff console |
| `apps/api` | Go API: gateway, inventory/holds, pricing, bookings, payments (mock), enquiries and AI |
| `apps/api/seed/data` | Seed data as language-neutral JSON fixtures. Derived values (nightly rates, sold counts, totals) are computed by the seeder, not stored here |
| `contract` | OpenAPI contract, written by hand. Every API test response is validated against it, and the web app's TypeScript types are generated from it |
| `infra` | Dockerfile and deploy config (planned) |
| `docs` | TDD, deployment guide, AI evaluation report, decision records |

Working on this repo? Read **[AGENTS.md](AGENTS.md)** first: product, architecture, coding standards, commands and what's next.

Setup and deployment: `docs/deployment.md` (in progress).
