// Generates apps/api/seed/data/enquiries.json with consistent received_at timestamps.
import { writeFileSync } from "node:fs";

const out = process.argv[2];
let n = 0;
const P = (i) => `+91 90000 1${String(i).padStart(4, "0")}`;

// [channel, name, subject|null, text, expected, tags, received_at override?, seed_state?]
const rows = [
  ["WEB_FORM", "Priya Nair", null, "Venue for my sister's wedding in Feb 2027, about 250 guests, 2 nights", { category: "WEDDING", guest_count: 250, event_month: "2027-02", nights: 2 }, ["brief_sample"]],
  ["WHATSAPP", "Arjun Menon", null, "Is a villa free 14-16 Nov for 2 adults and a child?", { category: "ROOM", check_in: "2026-11-14", check_out: "2026-11-16", adults: 2, children: 1 }, ["brief_sample"]],
  ["EMAIL", "Sneha Kulkarni", "January offsite enquiry", "Team of 45 wants a January offsite with a conference hall", { category: "OFFSITE", guest_count: 45, event_month: "2027-01" }, ["brief_sample"]],
  ["WHATSAPP", "Rahul Verma", null, "Hi, 21 Nov ko 2 raat ke liye room milega? Hum 2 log hain, breakfast ke saath kitna hoga?", { category: "ROOM", check_in: "2026-11-21", check_out: "2026-11-23", adults: 2 }, ["hinglish"]],
  ["EMAIL", "Vikram Joshi", "Very disappointed", "Stayed last weekend in room 214. Hot water didn't work both mornings and nobody came despite 3 calls to reception. Very disappointed for the price we paid.", { category: "COMPLAINT" }, []],
  ["WEB_FORM", "Lakshmi Raman", null, "Planning a trip for Christmas with my parents. Need 2 rooms from 24th to 27th Dec, 4 adults in total. Do you have estate view rooms?", { category: "ROOM", check_in: "2026-12-24", check_out: "2026-12-27", adults: 4, rooms: 2 }, ["peak_season"]],
  ["EMAIL", "Aisha Khan", "Destination wedding packages", "We're looking at destination wedding venues for Nov/Dec 2027. Roughly 180 guests, need about 40 rooms for 2 nights: mehendi, wedding and reception. Can you share packages?", { category: "WEDDING", guest_count: 180, event_month: null }, ["ambiguous_date"]],
  ["WHATSAPP", "Unknown", null, "price?", { category: "OTHER" }, ["vague"]],
  ["EMAIL", "RankBoost Media", "Get #1 on Google in 7 days!!!", "Boost your hotel's Google ranking to #1 in 7 days! SEO packages from $99. Reply YES to get started.", { category: "OTHER" }, ["spam"]],
  ["WEB_FORM", "Nikhil Bhat", null, "HR from a Bengaluru fintech here. 28 people, 2 nights in mid-March, need a hall for workshops and some team activities like a plantation trek. Budget around 12 lakh all inclusive.", { category: "OFFSITE", guest_count: 28, event_month: "2027-03", budget_inr: 1200000, budget_basis: "TOTAL" }, ["budget_lakh"]],
  ["EMAIL", "Farhan Ali", "Card statement question", "Just wondering if it's normal that we were charged twice for the dinner on 20 Oct? I can see two entries on my card statement.", { category: "COMPLAINT" }, ["disguised_complaint", "billing"]],
  ["WHATSAPP", "Kavya Reddy", null, "Pool villa available for new year? 30 Dec to 2 Jan, 2 couples", { category: "ROOM", check_in: "2026-12-30", check_out: "2027-01-02", adults: 4 }, ["peak_season"]],
  ["WHATSAPP", "Suresh Agarwal", null, "Bhaiya meri beti ki shaadi hai April 2027 mein, around 300 log. Lawn aur 30 rooms chahiye 2 din ke liye. Rate batao", { category: "WEDDING", guest_count: 300, event_month: "2027-04", nights: 2 }, ["hinglish"]],
  ["EMAIL", "Gauri Deshpande", "Family stay in December", "Hello, we're a family of 5 (3 adults, 2 kids aged 6 and 9) visiting from Pune, 8-11 December. Which room would fit us?", { category: "ROOM", check_in: "2026-12-08", check_out: "2026-12-11", adults: 3, children: 2 }, ["occupancy_fit"]],
  ["WEB_FORM", "Sanjay Pillai", null, "The staff at the restaurant were rude to my elderly father when he asked for less spicy food. I expected better hospitality.", { category: "COMPLAINT" }, []],
  ["EMAIL", "Ritu Malhotra", "Leadership retreat, Feb", "Leadership retreat for 12 senior managers, 3 nights, first week of February. Need privacy, good wifi and a boardroom. Can we book all the pool villas and heritage suites?", { category: "OFFSITE", secondary_category: "ROOM", guest_count: 12, event_month: "2027-02", nights: 3 }, ["mixed_intent"]],
  ["WHATSAPP", "Dev Sharma", null, "Do you allow dogs? Want to come with my labrador for 2 nights next weekend", { category: "ROOM", check_in: "2026-11-07", check_out: "2026-11-09" }, ["relative_date", "pets"], "2026-11-01T18:42:00+05:30"],
  ["EMAIL", "Tanya Kapoor", "Collaboration proposal", "Hi, I'm a travel blogger with 85k followers on Instagram. I'd love to collaborate on a complimentary stay in exchange for content.", { category: "OTHER" }, ["influencer"]],
  ["WEB_FORM", "Aditya Rao", null, "Honeymoon! Arriving 18 Jan, leaving 22 Jan. Something private with a nice view, budget ~80k for the stay.", { category: "ROOM", check_in: "2027-01-18", check_out: "2027-01-22", adults: 2, budget_inr: 80000, budget_basis: "TOTAL" }, []],
  ["EMAIL", "Neha Saxena", "Refund still pending: KS-4M8Q2T", "Booking ref KS-4M8Q2T: I cancelled 10 days before arrival and still haven't received my refund after 3 weeks. Please escalate.", { category: "COMPLAINT" }, ["refund"]],
  ["WHATSAPP", "Manoj Tiwari", null, "Sir hamari company ka annual offsite hai Jan end, 60 log, conference hall + stay chahiye. Quote bhejo", { category: "OFFSITE", guest_count: 60, event_month: "2027-01" }, ["hinglish"]],
  ["EMAIL", "Isabel Fernandes", "Intimate wedding, 12 March 2027", "Enquiring about a small, intimate wedding: 60 guests at the Valley Deck on 12 March 2027. We would need 20 rooms for 2 nights.", { category: "WEDDING", guest_count: 60, event_month: "2027-03", nights: 2 }, []],
  ["WHATSAPP", "Ravi", null, "2 rooms for 15th nov one night", { category: "ROOM", check_in: "2026-11-15", check_out: "2026-11-16", rooms: 2 }, ["terse"]],
  ["WEB_FORM", "Emma Wilson", null, "Do you sell your estate coffee online? We loved the filter coffee during our stay.", { category: "OTHER" }, ["general_question"]],
  ["EMAIL", "Pooja Hegde", "Feedback and a new booking", "We loved the stay overall, but the room smelled of damp and there were ants near the bed. Also, can I book again for Feb with a different room?", { category: "COMPLAINT", secondary_category: "ROOM" }, ["mixed_intent"]],
  ["WHATSAPP", "Prakash Gowda", null, "Namaskara, Dec 5 ge 2 days ge room beku, 2 adults. Rate eshtu?", { category: "ROOM", check_in: "2026-12-05", check_out: "2026-12-07", adults: 2 }, ["kannada_mix"]],
  ["EMAIL", "Shalini Dutta", "Board meeting and field visit", "We're an NGO planning a 2-day board meeting and field visit for 18 people around 20 Nov. Do you offer non-profit rates?", { category: "OFFSITE", guest_count: 18, event_month: "2026-11" }, []],
  ["WEB_FORM", "Studio Frames", null, "Pre-wedding photoshoot at the estate in December, a crew of 6, 1 day. Is that allowed and what are the charges?", { category: "WEDDING", guest_count: 6, event_month: "2026-12" }, ["edge_case"]],
  ["EMAIL", "Anil Kumar", "Application: Front Office Associate", "I'd like to apply for a front office position. My CV is attached.", { category: "OTHER" }, ["job"]],
  ["WHATSAPP", "Rekha Singh", null, "Kal raat 11 baje tak party wale side se loud music chal raha tha, bacche so nahi paye. Kya ye normal hai?", { category: "COMPLAINT" }, ["hinglish", "disguised_complaint"]],
  ["EMAIL", "Corporate Travel Desk", "3 rooms for directors, 9-11 Nov", "Corporate booking: 3 Estate View Rooms for our directors, 9-11 Nov. A GST invoice in the company name is required.", { category: "ROOM", check_in: "2026-11-09", check_out: "2026-11-11", rooms: 3 }, ["gst_invoice"]],
  ["WEB_FORM", "Harsh Vardhan", null, "Weekend getaway, any availability this Friday to Sunday?", { category: "ROOM", check_in: "2026-11-06", check_out: "2026-11-08" }, ["relative_date"], "2026-11-02T09:05:00+05:30"],
  ["EMAIL", "Mehak Arora", "Extra rooms for our wedding", "Our wedding is booked with you on 14-15 Jan (buyout). Can we add 10 more rooms for guests arriving on the 13th?", { category: "WEDDING", secondary_category: "ROOM", check_in: "2027-01-13", check_out: "2027-01-14", rooms: 10 }, ["existing_event"]],
  ["WHATSAPP", "Unknown", null, "ok thanks", { category: "OTHER" }, ["vague"]],
  ["EMAIL", "Ashwin Prabhu", "Team outing, December", "Need a quote for a 1-day team outing (no stay) for 35 people: lunch, plantation walk and coffee tasting, any Saturday in December.", { category: "OFFSITE", guest_count: 35, event_month: "2026-12" }, ["day_event"]],
  ["WEB_FORM", "Varun Nair", null, "Your website showed a villa available but after paying, the booking failed and the money was debited. Payment id pay_mock_88123.", { category: "COMPLAINT" }, ["payment"]],
  ["EMAIL", "Sunita Menon", "Accessibility question", "Is the heritage suite suitable for a wheelchair user? I want to book 2-5 Feb for my mother and me.", { category: "ROOM", check_in: "2027-02-02", check_out: "2027-02-05", adults: 2 }, ["accessibility"]],
  ["WHATSAPP", "Rohit Jain", null, "Engagement function for 120 guests in Jan, Valley Deck possible?", { category: "WEDDING", guest_count: 120, event_month: "2027-01" }, []],
  ["EMAIL", "Comfort Linen Co.", "Wholesale linen offer", "Dear Sir/Madam, we supply premium bedsheets and towels at wholesale prices. Catalogue attached.", { category: "OTHER" }, ["vendor"]],
  ["WEB_FORM", "Siddharth Rao", null, "We're 6 friends and want 3 rooms for 2 nights around Republic Day (Jan 25-27).", { category: "ROOM", check_in: "2027-01-25", check_out: "2027-01-27", adults: 6, rooms: 3 }, []],
  ["EMAIL", "Deepa Krishnan", "Charged for breakfast", "I booked a breakfast-included rate but was asked to pay for breakfast at checkout. This is not acceptable.", { category: "COMPLAINT" }, ["billing"]],
  ["WHATSAPP", "Imran Shaikh", null, "Bhai 2 adult 1 bachha, 28 se 30 Nov, villa ka rate kya hai?", { category: "ROOM", check_in: "2026-11-28", check_out: "2026-11-30", adults: 2, children: 1 }, ["hinglish"]],
  ["EMAIL", "Laura Becker", "Global team summit, April 2027", "We are planning a global team summit for 70 delegates from 8 countries, 4 nights in late April 2027. We need a full-day hall, an interpretation-friendly setup and airport transfers from Bengaluru.", { category: "OFFSITE", guest_count: 70, event_month: "2027-04", nights: 4 }, ["foreign"]],
  ["WEB_FORM", "Kabir Malhotra", null, "Destination wedding for 400 guests. We would need the entire property for 3 nights in December 2027. Budget is flexible.", { category: "WEDDING", guest_count: 400, event_month: "2027-12", nights: 3 }, ["buyout"]],
  ["EMAIL", "Jonas Müller", "Stay in February", "Hello from Germany! We are 2 adults travelling in India in February. Could we stay 3 nights from 14 Feb? Do you need our passport details now?", { category: "ROOM", check_in: "2027-02-14", check_out: "2027-02-17", adults: 2 }, ["foreign", "passport"]],
  ["WHATSAPP", "Geeta Bose", null, "The driver you arranged never showed up at Kadur station. We waited 1.5 hours and had to take an auto.", { category: "COMPLAINT" }, []],
  ["EMAIL", "Ankit Goel", "December weather", "What's the weather like in December? Should we carry woollens?", { category: "OTHER" }, ["general_question"]],
  ["WEB_FORM", "Batch of 2006", null, "School alumni reunion: 50 people, one evening dinner and 20 rooms for the night of 19 Dec.", { category: "OFFSITE", secondary_category: "ROOM", guest_count: 50, event_month: "2026-12", check_in: "2026-12-19", check_out: "2026-12-20", rooms: 20 }, ["edge_case"]],
  ["EMAIL", "Swati Mishra", "Change dates: KS-9P2L7D", "Can I change my booking KS-9P2L7D from 20-22 Nov to 27-29 Nov?", { category: "ROOM", check_in: "2026-11-27", check_out: "2026-11-29" }, ["modification"]],
  ["WHATSAPP", "Unknown", null, "Congratulations! You have won Rs 25,00,000 in the KBC lottery. Send your bank details to claim.", { category: "OTHER" }, ["spam"]],
  ["EMAIL", "Rajesh Iyer", "Misleading photos", "The pool villa photos on your website show a big infinity pool, but the actual pool was a small plunge pool. We felt misled.", { category: "COMPLAINT" }, []],
  ["WEB_FORM", "Nisha Patel", null, "Looking for something under 10k per night for 2 people, 3 nights in mid Jan.", { category: "ROOM", adults: 2, budget_inr: 10000, budget_basis: "PER_NIGHT" }, ["budget_per_night", "ambiguous_date"]],
  ["EMAIL", "Ramesh Chandra", "विवाह स्थल की जानकारी", "नमस्ते, हम मार्च 2027 में अपने बेटे की शादी के लिए जगह देख रहे हैं। लगभग 200 मेहमान होंगे। कृपया पैकेज भेजें।", { category: "WEDDING", guest_count: 200, event_month: "2027-03" }, ["hindi"]],
  ["WHATSAPP", "Kiran", null, "corporate offsite 25 pax feb", { category: "OFFSITE", guest_count: 25, event_month: "2027-02" }, ["terse"]],
  ["EMAIL", "Mohan Das", "THIRD EMAIL: damaged luggage", "This is my third email. If I don't get a response about the damaged luggage by Friday, I will post reviews everywhere and take legal action.", { category: "COMPLAINT" }, ["escalation"]],
  ["WEB_FORM", "Arvind Kumar", null, "Anniversary surprise for my wife: 1 night on 12 Dec in your best room. Can you arrange a cake and flowers?", { category: "ROOM", check_in: "2026-12-12", check_out: "2026-12-13", adults: 2 }, ["special_request"]],
  ["EMAIL", "Zoya Siddiqui", "EV charging", "Do you have an EV charger at the property?", { category: "OTHER" }, ["general_question"]],
  ["WHATSAPP", "Naveen", null, "Need 1 room tonight, 2 adults, arriving around 9pm", { category: "ROOM", check_in: "2026-11-02", check_out: "2026-11-03", adults: 2 }, ["same_day", "relative_date"], "2026-11-02T13:20:00+05:30"],
  ["EMAIL", "Priyanka Sethi", "Sales conference + engagement", "We'd like to host a 2-day sales conference (40 pax) and our MD's daughter's engagement on the second evening for 100 guests. Can these be combined?", { category: "OFFSITE", secondary_category: "WEDDING", guest_count: 40 }, ["mixed_intent"]],
  ["WEB_FORM", "S. Rao", null, "Hi, please call me back.", { category: "OTHER" }, ["vague"]],
];

// Older enquiries were already handled, which gives the dashboard a real first-reply metric.
const handled = { 0: 22, 2: 41, 4: 9, 9: 55, 10: 18, 12: 64, 14: 12, 15: 38, 19: 27, 20: 47, 21: 33, 24: 15, 26: 72, 27: 51, 29: 8, 30: 24 };

const start = Date.parse("2026-10-23T08:15:00+05:30");
const step = (Date.parse("2026-11-02T12:00:00+05:30") - start) / rows.length;
const iso = (ms) => new Date(ms + 5.5 * 3600e3).toISOString().replace("Z", "+05:30").replace(".000", "");

const enquiries = rows.map(([channel, name, subject, text, expected, tags, at], i) => {
  n++;
  const ref = `ENQ-${4410 + i}`;
  const slug = name.toLowerCase().normalize("NFKD").replace(/[^a-z0-9]+/g, ".").replace(/^\.|\.$/g, "") || "sender";
  const sender = { name };
  if (channel === "WHATSAPP") sender.phone = P(i + 1);
  else if (channel === "EMAIL") sender.email = `${slug}@example.test`;
  else { sender.email = `${slug}@example.test`; sender.phone = P(i + 1); }
  const received_at = at ?? iso(start + Math.round(step * i + ((i * 7919) % 2400) * 1000));
  const seed_state = handled[i] != null
    ? { status: i === 4 || i === 10 ? "IN_PROGRESS" : "REPLIED", first_response_minutes: handled[i] }
    : { status: "NEW" };
  return { ref, channel, received_at, sender, subject: subject ?? undefined, text, expected, tags, seed_state };
});

enquiries.sort((a, b) => a.received_at.localeCompare(b.received_at));

writeFileSync(out, JSON.stringify({
  _note: "60 realistic enquiries. 'expected' is the human label used by the AI evaluation (cmd/eval). ENQ-4410/4411/4412 are verbatim from the brief. 'seed_state' gives the inbox some history; AI fields are computed by the classifier when seeding, not stored here. Senders are fictional (.test domains, 90000-series numbers).",
  reference_now: "2026-11-02T14:00:00+05:30",
  enquiries,
}, null, 2) + "\n");
console.log(`wrote ${n} enquiries`);
