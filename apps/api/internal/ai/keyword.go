package ai

import (
	"regexp"
	"strconv"

	"kaapi/api/internal/domain"
)

// The keyword classifier is the always-on second opinion and the fallback when the LLM is
// unavailable. It covers English, common Hinglish and Hindi, and Kannada-English words
// seen in the resort's messages.
var keywordRules = []struct {
	category domain.Category
	weight   float64
	words    []string
}{
	{domain.CategoryComplaint, 3, []string{"complain", "disappoint", "rude", "refund", "not acceptable", "unacceptable", "never showed",
		"didn't work", "did not work", "dirty", "damp", "ants", "charged twice", "misled", "legal action", "third email", "escalate",
		"loud music", "worst", "terrible", "not happy", "so nahi paye", "kharab", "bekaar", "shikayat", "damaged"}},
	{domain.CategoryWedding, 2.5, []string{"wedding", "shaadi", "shadi", "शादी", "vivah", "विवाह", "mehendi", "sangeet", "reception",
		"engagement", "pre-wedding", "bride", "groom", "mandap", "baraat"}},
	{domain.CategoryOffsite, 2.5, []string{"offsite", "off-site", "conference", "team outing", "retreat", "summit", "workshop",
		"corporate", "company", "delegates", "board meeting", "pax", "team of", "employees", "reunion", "hall"}},
	{domain.CategoryRoom, 1, []string{"room", "villa", "stay", "night", "raat", "book", "booking", "available", "availability",
		"check-in", "check in", "honeymoon", "anniversary", "weekend", "suite", "beku", "milega", "rate", "tonight"}},
	{domain.CategoryOther, 2, []string{"seo", "lottery", "won rs", "catalogue", "wholesale", "collaborat", "influencer", "followers",
		"cv", "apply for", "position", "ev charger", "weather", "call me back", "sell your", "bank details"}},
}

// keywordRes match each word at a word start, so "rate" doesn't fire on "corporate"
// but "complain" still matches "complaint". Non-Latin words match anywhere.
var keywordRes = func() [][]*regexp.Regexp {
	out := make([][]*regexp.Regexp, len(keywordRules))
	for i, rule := range keywordRules {
		for _, w := range rule.words {
			out[i] = append(out[i], regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}])`+regexp.QuoteMeta(w)))
		}
	}
	return out
}()

var (
	guestCountRe = regexp.MustCompile(`(?i)(\d{2,4})\s*(?:guests|people|pax|log|persons|delegates|मेहमान)`)
	adultsRe     = regexp.MustCompile(`(?i)(\d{1,2})\s*(?:adults?|adult)`)
	childRe      = regexp.MustCompile(`(?i)(\d{1,2})\s*(?:kids?|child(?:ren)?|bachh?[ae])|\ba child\b|\bek bachha\b`)
)

// KeywordClassify scores categories by matched words. It's deliberately simple and
// predictable: its job is to be a cheap, explainable cross-check.
func KeywordClassify(text string) Classification {
	scores := map[domain.Category]float64{}
	for i, rule := range keywordRules {
		for _, re := range keywordRes[i] {
			if re.MatchString(text) {
				scores[rule.category] += rule.weight
			}
		}
	}
	best, second := domain.CategoryOther, domain.Category("")
	var bestScore, secondScore float64
	for _, c := range domain.Categories {
		s := scores[c]
		switch {
		case s > bestScore:
			second, secondScore = best, bestScore
			best, bestScore = c, s
		case s > secondScore:
			second, secondScore = c, s
		}
	}
	out := Classification{Category: best, Language: "unknown"}
	total := 0.0
	for _, s := range scores {
		total += s
	}
	switch {
	case bestScore == 0:
		out.Category, out.Confidence = domain.CategoryOther, 0.2
	default:
		out.Confidence = 0.35 + 0.5*(bestScore/total)
	}
	if second != "" && secondScore >= 2 && second != best {
		s := second
		out.Secondary = &s
	}
	if m := guestCountRe.FindStringSubmatch(text); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			out.Extracted.GuestCount = &n
		}
	}
	if m := adultsRe.FindStringSubmatch(text); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			out.Extracted.Adults = &n
		}
	}
	if m := childRe.FindStringSubmatch(text); m != nil {
		n := 1
		if m[1] != "" {
			n, _ = strconv.Atoi(m[1])
		}
		out.Extracted.Children = &n
	}
	return out
}
