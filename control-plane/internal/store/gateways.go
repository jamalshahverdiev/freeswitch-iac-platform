package store

import (
	"context"
	"errors"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateGateway(ctx context.Context, g *models.Gateway) error {
	g.ID = uuid.NewString()
	if g.Params == nil {
		g.Params = map[string]string{}
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO gateways (id, name, profile, enabled, username, password, realm, proxy, register, params)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at, updated_at`,
		g.ID, g.Name, g.Profile, g.Enabled, g.Username, g.Password, g.Realm, g.Proxy, g.Register, g.Params,
	).Scan(&g.CreatedAt, &g.UpdatedAt)
	if isUniqueViolation(err) {
		return ErrAlreadyExists
	}
	return err
}

func (s *Store) GetGateway(ctx context.Context, profile, name string) (*models.Gateway, error) {
	var g models.Gateway
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, profile, enabled, COALESCE(username,''), COALESCE(password,''),
		       COALESCE(realm,''), proxy, register, params, created_at, updated_at
		FROM gateways WHERE profile = $1 AND name = $2`, profile, name,
	).Scan(&g.ID, &g.Name, &g.Profile, &g.Enabled, &g.Username, &g.Password,
		&g.Realm, &g.Proxy, &g.Register, &g.Params, &g.CreatedAt, &g.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *Store) ListGateways(ctx context.Context, profileFilter string) ([]models.Gateway, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, profile, enabled, COALESCE(username,''), COALESCE(password,''),
		       COALESCE(realm,''), proxy, register, params, created_at, updated_at
		FROM gateways
		WHERE ($1 = '' OR profile = $1)
		ORDER BY profile, name`, profileFilter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Gateway{}
	for rows.Next() {
		var g models.Gateway
		if err := rows.Scan(&g.ID, &g.Name, &g.Profile, &g.Enabled, &g.Username, &g.Password,
			&g.Realm, &g.Proxy, &g.Register, &g.Params, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) UpdateGateway(ctx context.Context, profile, name string, g *models.Gateway) error {
	if g.Params == nil {
		g.Params = map[string]string{}
	}
	err := s.pool.QueryRow(ctx, `
		UPDATE gateways
		SET enabled = $3, username = $4, password = $5, realm = $6, proxy = $7, register = $8, params = $9, updated_at = NOW()
		WHERE profile = $1 AND name = $2
		RETURNING id, name, profile, created_at, updated_at`,
		profile, name, g.Enabled, g.Username, g.Password, g.Realm, g.Proxy, g.Register, g.Params,
	).Scan(&g.ID, &g.Name, &g.Profile, &g.CreatedAt, &g.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Store) DeleteGateway(ctx context.Context, profile, name string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM gateways WHERE profile = $1 AND name = $2`, profile, name)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
