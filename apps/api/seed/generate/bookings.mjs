// Generates apps/api/seed/data/bookings.json, then validates every CONFIRMED booking
// against the allotment rules (nightly allotment, stop-sell, min stay, closed-to-arrival).
import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const dir = process.argv[2];
const rules = JSON.parse(readFileSync(join(dir, "rates_rules.json"), "utf8"));
const plans = JSON.parse(readFileSync(join(dir, "rate_plans.json"), "utf8")).plans;
const types = JSON.parse(readFileSync(join(dir, "room_types.json"), "utf8")).room_types;

const T = { P: "plantation-room", E: "estate-view-room", H: "heritage-suite", C: "coffee-villa", V: "pool-villa" };
// ref, type, plan, in, out, rooms, adults, children, guest, country, status, promo, created, request
const rows = [
  ["KS-2A7F4Q", "P", "BB-FLEX", "2026-11-01", "2026-11-03", 1, 2, 0, "Meera Iyer", "IN", "CONFIRMED", null, "2026-10-12", null],
  ["KS-3B8G5R", "E", "RO-FLEX", "2026-11-02", "2026-11-04", 1, 2, 0, "Abhishek Rao", "IN", "CONFIRMED", null, "2026-10-20", null],
  ["KS-4C9H6S", "C", "BB-FLEX", "2026-11-06", "2026-11-09", 1, 3, 1, "Shreya Banerjee", "IN", "CONFIRMED", null, "2026-10-05", "Travelling with a beagle"],
  ["KS-5D2J7T", "P", "BB-NRF", "2026-11-06", "2026-11-08", 2, 4, 0, "Karan Mehta", "IN", "CONFIRMED", null, "2026-10-14", "Adjacent rooms if possible"],
  ["KS-6E3K8U", "V", "BB-FLEX", "2026-11-07", "2026-11-09", 1, 2, 0, "Oliver Grant", "GB", "CONFIRMED", null, "2026-09-28", null],
  ["KS-7F4L9V", "H", "BB-FLEX", "2026-11-07", "2026-11-10", 1, 2, 1, "Ananya Krishnan", "IN", "CONFIRMED", null, "2026-10-18", null],
  ["KS-8G5M2W", "E", "BB-FLEX", "2026-11-09", "2026-11-11", 1, 2, 0, "Vivek Sinha", "IN", "CONFIRMED", null, "2026-10-25", null],
  ["KS-9P2L7D", "P", "BB-FLEX", "2026-11-20", "2026-11-22", 1, 2, 0, "Swati Mishra", "IN", "CONFIRMED", null, "2026-10-21", null],
  ["KS-4M8Q2T", "H", "BB-FLEX", "2026-11-13", "2026-11-15", 1, 2, 0, "Neha Saxena", "IN", "REFUND_PENDING", null, "2026-09-30", null],
  ["KS-2N3R4X", "C", "RO-FLEX", "2026-11-14", "2026-11-16", 1, 2, 0, "Harini Subramanian", "IN", "CONFIRMED", null, "2026-10-27", null],
  ["KS-3P4S5Y", "P", "RO-FLEX", "2026-11-14", "2026-11-15", 1, 2, 0, "Gaurav Saini", "IN", "CONFIRMED", null, "2026-10-30", null],
  ["KS-4Q5T6Z", "E", "BB-NRF", "2026-11-21", "2026-11-23", 1, 2, 0, "Tara Fernandes", "IN", "CONFIRMED", "DIRECT10", "2026-10-29", null],
  ["KS-5R6U7A", "V", "BB-FLEX", "2026-11-21", "2026-11-24", 1, 2, 2, "Rakesh Nambiar", "IN", "CONFIRMED", null, "2026-10-16", "Pool heated for the kids, please"],
  ["KS-6S7V8B", "H", "RO-FLEX", "2026-11-27", "2026-11-29", 1, 2, 0, "Emily Clarke", "US", "CONFIRMED", null, "2026-10-31", null],
  ["KS-7T8W9C", "C", "BB-FLEX", "2026-11-28", "2026-12-02", 1, 2, 1, "Siddharth Joshi", "IN", "CONFIRMED", "STAYLONGER", "2026-10-22", null],
  ["KS-8U9X2D", "P", "BB-FLEX", "2026-12-05", "2026-12-07", 1, 2, 0, "Nandini Rao", "IN", "CONFIRMED", null, "2026-11-01", null],
  ["KS-9V2Y3E", "P", "BB-FLEX", "2026-12-12", "2026-12-14", 2, 4, 0, "Akash Patil", "IN", "CONFIRMED", null, "2026-10-28", null],
  ["KS-2W3Z4F", "E", "BB-FLEX", "2026-12-12", "2026-12-13", 1, 2, 0, "Ishaan Chopra", "IN", "CONFIRMED", null, "2026-10-30", "Anniversary"],
  ["KS-3X4A5G", "V", "BB-NRF", "2026-12-18", "2026-12-21", 1, 2, 0, "Sophie Martin", "FR", "CONFIRMED", "DIRECT10", "2026-10-19", null],
  ["KS-4Y5B6H", "P", "BB-FLEX", "2026-12-24", "2026-12-27", 2, 4, 0, "Manish Agarwal", "IN", "CONFIRMED", null, "2026-09-15", null],
  ["KS-5Z6C7J", "E", "BB-FLEX", "2026-12-24", "2026-12-27", 1, 2, 0, "Ritika Sharma", "IN", "CONFIRMED", null, "2026-09-20", null],
  ["KS-6A7D8K", "C", "BB-FLEX", "2026-12-24", "2026-12-28", 1, 4, 0, "Daniel Thomas", "IN", "CONFIRMED", null, "2026-08-30", null],
  ["KS-7B8E9L", "C", "BB-NRF", "2026-12-29", "2027-01-02", 1, 2, 2, "Pallavi Kamath", "IN", "CONFIRMED", null, "2026-09-02", null],
  ["KS-8C9F2M", "V", "BB-FLEX", "2026-12-30", "2027-01-02", 1, 4, 0, "Aryan Khanna", "IN", "CONFIRMED", null, "2026-08-11", "New Year's Eve dinner in the pavilion"],
  ["KS-9D2G3N", "H", "BB-FLEX", "2026-12-30", "2027-01-01", 1, 2, 0, "Lucy Evans", "GB", "CONFIRMED", null, "2026-09-25", null],
  ["KS-2E3H4P", "P", "BB-NRF", "2026-12-30", "2027-01-01", 3, 6, 0, "Rohan Kulkarni", "IN", "CONFIRMED", null, "2026-10-02", "Group of friends, rooms close together"],
  ["KS-3F4J5Q", "E", "BB-FLEX", "2026-12-30", "2027-01-02", 1, 2, 0, "Anjali Varma", "IN", "CONFIRMED", null, "2026-10-08", null],
  ["KS-4G5K6R", "P", "RO-FLEX", "2027-01-09", "2027-01-11", 1, 2, 0, "Sameer Kaul", "IN", "CONFIRMED", null, "2026-10-26", null],
  ["KS-5H6L7S", "H", "BB-FLEX", "2027-01-23", "2027-01-26", 1, 2, 0, "Divya Menon", "IN", "CONFIRMED", null, "2026-10-17", null],
  ["KS-6J7M8T", "E", "BB-FLEX", "2027-01-23", "2027-01-25", 2, 4, 0, "Pranav Desai", "IN", "CONFIRMED", null, "2026-10-24", null],
  ["KS-7K8N9U", "C", "BB-FLEX", "2027-02-12", "2027-02-15", 1, 2, 0, "Hannah Weber", "DE", "CONFIRMED", null, "2026-10-09", null],
  ["KS-8L9P2V", "V", "BB-FLEX", "2027-02-13", "2027-02-15", 1, 2, 0, "Rahul Bhatia", "IN", "CONFIRMED", null, "2026-10-29", "Proposal planned, please keep it quiet"],
  ["KS-9M2Q3W", "P", "BB-FLEX", "2027-02-26", "2027-03-01", 1, 2, 1, "Meera Iyer", "IN", "CONFIRMED", null, "2026-11-01", null],
  ["KS-2N3R4Y", "H", "BB-NRF", "2027-03-05", "2027-03-08", 1, 2, 0, "Kunal Shah", "IN", "CONFIRMED", "DIRECT10", "2026-10-30", null],
  ["KS-3P4S5Z", "C", "BB-FLEX", "2027-04-10", "2027-04-14", 1, 4, 0, "Vandana Reddy", "IN", "CONFIRMED", "STAYLONGER", "2026-10-31", null],
  ["KS-4Q5T6A", "C", "BB-FLEX", "2026-11-14", "2026-11-16", 1, 2, 1, "Varun Nair", "IN", "PAYMENT_FAILED", null, "2026-10-26", null],
  ["KS-5R6U7B", "P", "RO-FLEX", "2026-11-21", "2026-11-22", 1, 2, 0, "Nitin Gupta", "IN", "EXPIRED", null, "2026-10-28", null],
  ["KS-6S7V8C", "E", "BB-FLEX", "2026-12-24", "2026-12-26", 1, 2, 0, "Aarti Joshi", "IN", "EXPIRED", null, "2026-10-30", null],
  ["KS-7T8W9D", "H", "BB-FLEX", "2026-11-20", "2026-11-22", 1, 2, 0, "Mohit Arora", "IN", "CANCELLED", null, "2026-10-10", null],
  ["KS-8U9X2E", "V", "BB-FLEX", "2027-01-16", "2027-01-18", 1, 2, 0, "Claire Dubois", "FR", "REFUNDED", null, "2026-09-18", null],
];

const slug = (s) => s.toLowerCase().replace(/[^a-z0-9]+/g, ".").replace(/^\.|\.$/g, "");
const bookings = rows.map(([ref, t, plan, check_in, check_out, rooms, adults, children, name, country, status, promo, created, req], i) => ({
  ref, room_type: T[t], rate_plan: plan, check_in, check_out, rooms, adults, children,
  guest: {
    name,
    email: name === "Meera Iyer" ? "guest@example.test" : `${slug(name)}@example.test`,
    phone: `+91 90000 2${String(i + 1).padStart(4, "0")}`,
    country,
  },
  status, promo_code: promo, created_at: `${created}T${String(9 + (i % 10)).padStart(2, "0")}:${String((i * 13) % 60).padStart(2, "0")}:00+05:30`,
  special_requests: req,
  ...(ref === "KS-4Q5T6A" ? { payment: { provider_payment_id: "pay_mock_88123", status: "FAILED", failure_reason: "Bank timeout after debit (simulated)" } } : {}),
}));

// ---- validation ----
const day = 864e5, D = (s) => Date.parse(s + "T00:00:00Z"), S = (ms) => new Date(ms).toISOString().slice(0, 10);
const nightsOf = (a, b) => { const r = []; for (let t = D(a); t < D(b); t += day) r.push(S(t)); return r; };
const inRange = (d, f, t) => d >= f && d <= t;
const errors = [];
const sold = {};
const keys = Object.fromEntries(types.map((t) => [t.slug, t.total_keys]));
const allot = (type, d) => {
  let a = rules.website_allotment[type], stop = false;
  for (const o of rules.allotment_overrides) if ((o.room_type === "*" || o.room_type === type) && inRange(d, o.from, o.to)) { if (o.stop_sell) stop = true; if (o.allotment != null) a = o.allotment; }
  return { a, stop };
};
for (const b of bookings) {
  const nights = nightsOf(b.check_in, b.check_out);
  const plan = plans.find((p) => p.code === b.rate_plan);
  const type = types.find((t) => t.slug === b.room_type);
  if (!plan || !type) errors.push(`${b.ref}: unknown plan/type`);
  if (nights.length < plan.min_nights) errors.push(`${b.ref}: plan min ${plan.min_nights} nights`);
  if (b.adults > type.max_adults * b.rooms || b.children > type.max_children * b.rooms) errors.push(`${b.ref}: occupancy`);
  if (b.status !== "CONFIRMED") continue;
  for (const s of rules.seasons) {
    if (inRange(b.check_in, s.from, s.to) && s.min_stay && nights.length < s.min_stay) errors.push(`${b.ref}: season min stay ${s.min_stay}`);
    if (s.closed_to_arrival?.includes(b.check_in)) errors.push(`${b.ref}: closed to arrival`);
  }
  for (const d of nights) {
    const { a, stop } = allot(b.room_type, d);
    if (stop) errors.push(`${b.ref}: stop-sell on ${d}`);
    const k = `${b.room_type}|${d}`;
    sold[k] = (sold[k] || 0) + b.rooms;
    if (sold[k] > a) errors.push(`${b.ref}: OVERBOOKED ${k} sold ${sold[k]} > allotment ${a}`);
    if (a > keys[b.room_type]) errors.push(`allotment exceeds keys ${k}`);
  }
}
const refs = new Set(bookings.map((b) => b.ref));
if (refs.size !== bookings.length) errors.push("duplicate refs");
if (Object.values(keys).reduce((x, y) => x + y) !== 76) errors.push("keys != 76");

if (errors.length) { console.error(errors.join("\n")); process.exit(1); }
writeFileSync(join(dir, "bookings.json"), JSON.stringify({
  _note: "Booking inputs only. The seeder prices each one with the real pricing engine and derives night_inventory.sold from CONFIRMED bookings, so totals and inventory can never disagree with the code. This file is validated against the allotment rules when generated.",
  bookings,
}, null, 2) + "\n");
const tight = Object.entries(sold).filter(([k, v]) => { const [t, d] = k.split("|"); return v >= allot(t, d).a; }).map(([k]) => k);
console.log(`ok: ${bookings.length} bookings, ${bookings.filter((b) => b.status === "CONFIRMED").length} confirmed; sold-out nights:`, tight.length);
console.log(tight.join("  "));
