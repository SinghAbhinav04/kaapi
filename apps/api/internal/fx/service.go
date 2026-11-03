package fx

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaapi/api/internal/domain"
	"kaapi/api/internal/platform/db"
)

type Service struct {
	pool    *pgxpool.Pool
	http    *http.Client
	baseURL string
}

// NewService reads rates from baseURL (empty: Frankfurter).
func NewService(pool *pgxpool.Pool, baseURL string) *Service {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Service{pool: pool, http: &http.Client{Timeout: requestTimeout}, baseURL: baseURL}
}

// Refresh fetches today's rates and stores them, returning how many were updated. The
// network call happens before the transaction opens.
func (s *Service) Refresh(ctx context.Context) (int, error) {
	rates, err := fetch(ctx, s.http, s.baseURL)
	if err != nil {
		return 0, err
	}
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		for _, r := range rates {
			if _, err := tx.Exec(ctx, `INSERT INTO fx_rates (currency, inr_per_unit_micros, rate_date, fetched_at)
				VALUES ($1, $2, $3, now())
				ON CONFLICT (currency) DO UPDATE SET inr_per_unit_micros = EXCLUDED.inr_per_unit_micros,
					rate_date = EXCLUDED.rate_date, fetched_at = now()`, r.Currency, r.INRPerUnitMicros, r.Date); err != nil {
				return err
			}
		}
		return nil
	})
	return len(rates), err
}

// Rates returns every rate fresh enough to show. None at all is a normal answer: the site
// then shows INR only.
func (s *Service) Rates(ctx context.Context) ([]Rate, error) {
	rows, err := s.pool.Query(ctx, `SELECT currency, inr_per_unit_micros, rate_date, fetched_at FROM fx_rates
		WHERE fetched_at > now() - make_interval(secs => $1) ORDER BY currency`, FreshFor.Seconds())
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Rate, error) {
		var x Rate
		return x, r.Scan(&x.Currency, &x.INRPerUnitMicros, &x.Date, &x.FetchedAt)
	})
}

// Snapshot is the fresh rate for c inside the caller's transaction, or nil when there is
// none (INR itself, or rates gone stale). A booking stores it as what the guest saw.
func Snapshot(ctx context.Context, tx pgx.Tx, c domain.Currency) (*Rate, error) {
	if c == domain.CurrencyINR {
		return nil, nil
	}
	var r Rate
	err := tx.QueryRow(ctx, `SELECT currency, inr_per_unit_micros, rate_date, fetched_at FROM fx_rates
		WHERE currency = $1 AND fetched_at > now() - make_interval(secs => $2)`, c, FreshFor.Seconds()).
		Scan(&r.Currency, &r.INRPerUnitMicros, &r.Date, &r.FetchedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}
