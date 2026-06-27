package store

import (
	"context"
)

// PushSub is a browser Web Push subscription bound to an operator's extension.
type PushSub struct {
	Subject   string
	Domain    string
	Number    string
	Endpoint  string
	P256dh    string
	Auth      string
	UserAgent string
}

// SavePushSub upserts a subscription by endpoint (the endpoint is the natural
// key; the same browser re-subscribing returns the same endpoint).
func (s *Store) SavePushSub(ctx context.Context, p PushSub) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO push_subscriptions (subject, domain, number, endpoint, p256dh, auth, user_agent)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (endpoint) DO UPDATE SET
			subject = EXCLUDED.subject,
			domain  = EXCLUDED.domain,
			number  = EXCLUDED.number,
			p256dh  = EXCLUDED.p256dh,
			auth    = EXCLUDED.auth,
			user_agent = EXCLUDED.user_agent`,
		p.Subject, p.Domain, p.Number, p.Endpoint, p.P256dh, p.Auth, p.UserAgent)
	return err
}

// DeletePushSub removes a subscription by endpoint (called on unsubscribe or
// when the push service reports the endpoint is gone, 404/410).
func (s *Store) DeletePushSub(ctx context.Context, endpoint string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM push_subscriptions WHERE endpoint = $1`, endpoint)
	return err
}

// PushSubsForExtension returns every subscription registered for domain/number.
func (s *Store) PushSubsForExtension(ctx context.Context, domain, number string) ([]PushSub, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT subject, domain, number, endpoint, p256dh, auth, user_agent
		FROM push_subscriptions WHERE domain = $1 AND number = $2`, domain, number)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PushSub{}
	for rows.Next() {
		var p PushSub
		if err := rows.Scan(&p.Subject, &p.Domain, &p.Number, &p.Endpoint, &p.P256dh, &p.Auth, &p.UserAgent); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
