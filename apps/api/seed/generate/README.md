# Fixture generators

These scripts generate the larger fixtures and check them. The JSON they write is committed, so they don't need to run as part of the build.

```sh
node apps/api/seed/generate/enquiries.mjs apps/api/seed/data/enquiries.json
node apps/api/seed/generate/bookings.mjs  apps/api/seed/data      # fails if any CONFIRMED booking breaks allotment rules
```
