package fx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"kaapi/api/internal/domain"
)

// latest is Frankfurter's /latest answer: units of each currency per one unit of base.
type latest struct {
	Base  string                 `json:"base"`
	Date  string                 `json:"date"`
	Rates map[string]json.Number `json:"rates"`
}

// fetch asks for today's EUR-based rates and turns them into INR per unit. EUR is the ECB's
// own base, so crossing through it keeps the published precision (asking for base=INR gets
// rates rounded to a few significant digits).
func fetch(ctx context.Context, client *http.Client, baseURL string) ([]Rate, error) {
	symbols := make([]string, 0, len(domain.Currencies))
	for _, c := range domain.Currencies {
		if c != ecbBase {
			symbols = append(symbols, string(c))
		}
	}
	u := strings.TrimRight(baseURL, "/") + "/latest?" + url.Values{"symbols": {strings.Join(symbols, ",")}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fx: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("fx: read: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fx: HTTP %d: %.200s", res.StatusCode, body)
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var l latest
	if err := dec.Decode(&l); err != nil {
		return nil, fmt.Errorf("fx: decode: %w", err)
	}
	return toINR(l)
}

// toINR converts "X per EUR" into "INR per X" with exact rational arithmetic, rounding
// only once at the end.
func toINR(l latest) ([]Rate, error) {
	if l.Base != string(ecbBase) {
		return nil, fmt.Errorf("fx: expected base %s, got %q", ecbBase, l.Base)
	}
	date, err := time.Parse(time.DateOnly, l.Date)
	if err != nil {
		return nil, fmt.Errorf("fx: date %q: %w", l.Date, err)
	}
	inrPerEUR, ok := positiveRat(l.Rates[string(domain.CurrencyINR)])
	if !ok {
		return nil, fmt.Errorf("fx: no INR rate in the answer")
	}
	var out []Rate
	for _, c := range domain.Currencies {
		if c == domain.CurrencyINR {
			continue
		}
		perEUR := big.NewRat(1, 1) // EUR per EUR
		if c != ecbBase {
			if perEUR, ok = positiveRat(l.Rates[string(c)]); !ok {
				continue // not published today: keep yesterday's until it goes stale
			}
		}
		inr := new(big.Rat).Quo(inrPerEUR, perEUR)
		micros := roundRat(inr.Mul(inr, big.NewRat(MicrosPerUnit, 1)))
		if micros < 1 {
			continue
		}
		out = append(out, Rate{Currency: c, INRPerUnitMicros: micros, Date: date})
	}
	return out, nil
}

func positiveRat(n json.Number) (*big.Rat, bool) {
	r, ok := new(big.Rat).SetString(n.String())
	return r, ok && r.Sign() > 0
}

// roundRat rounds a positive rational half up to an int64.
func roundRat(r *big.Rat) int64 {
	num := new(big.Int).Mul(r.Num(), big.NewInt(2))
	num.Add(num, r.Denom())
	den := new(big.Int).Mul(r.Denom(), big.NewInt(2))
	return num.Quo(num, den).Int64()
}
