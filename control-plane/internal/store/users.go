package store

import (
	"context"
	"errors"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) domainID(ctx context.Context, name string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT id FROM domains WHERE name = $1`, name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return id, err
}

func (s *Store) CreateUser(ctx context.Context, u *models.User) error {
	domainID, err := s.domainID(ctx, u.Domain)
	if err != nil {
		return err
	}
	u.ID = uuid.NewString()
	if u.Params == nil {
		u.Params = map[string]string{}
	}
	if u.Variables == nil {
		u.Variables = map[string]string{}
	}
	err = s.pool.QueryRow(ctx, `
		INSERT INTO users (id, domain_id, number, enabled, params, variables)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at`,
		u.ID, domainID, u.Number, u.Enabled, u.Params, u.Variables,
	).Scan(&u.CreatedAt, &u.UpdatedAt)
	if isUniqueViolation(err) {
		return ErrAlreadyExists
	}
	return err
}

func (s *Store) GetUser(ctx context.Context, domain, number string) (*models.User, error) {
	var u models.User
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, d.name, u.number, u.enabled, u.params, u.variables, u.created_at, u.updated_at
		FROM users u JOIN domains d ON d.id = u.domain_id
		WHERE d.name = $1 AND u.number = $2`, domain, number,
	).Scan(&u.ID, &u.Domain, &u.Number, &u.Enabled, &u.Params, &u.Variables, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) ListUsers(ctx context.Context, domainFilter string) ([]models.User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, d.name, u.number, u.enabled, u.params, u.variables, u.created_at, u.updated_at
		FROM users u JOIN domains d ON d.id = u.domain_id
		WHERE ($1 = '' OR d.name = $1)
		ORDER BY d.name, u.number`, domainFilter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.User{}
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Domain, &u.Number, &u.Enabled, &u.Params, &u.Variables, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) UpdateUser(ctx context.Context, domain, number string, u *models.User) error {
	if u.Params == nil {
		u.Params = map[string]string{}
	}
	if u.Variables == nil {
		u.Variables = map[string]string{}
	}
	err := s.pool.QueryRow(ctx, `
		UPDATE users u
		SET enabled = $3, params = $4, variables = $5, updated_at = NOW()
		FROM domains d
		WHERE u.domain_id = d.id AND d.name = $1 AND u.number = $2
		RETURNING u.id, d.name, u.number, u.created_at, u.updated_at`,
		domain, number, u.Enabled, u.Params, u.Variables,
	).Scan(&u.ID, &u.Domain, &u.Number, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Store) DeleteUser(ctx context.Context, domain, number string) error {
	ct, err := s.pool.Exec(ctx, `
		DELETE FROM users u
		USING domains d
		WHERE u.domain_id = d.id AND d.name = $1 AND u.number = $2`, domain, number)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
