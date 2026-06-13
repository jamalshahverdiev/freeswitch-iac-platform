package store

import (
	"context"
	"errors"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23503"
	}
	return false
}

func (s *Store) CreateDomain(ctx context.Context, d *models.Domain) error {
	d.ID = uuid.NewString()
	if d.Variables == nil {
		d.Variables = map[string]string{}
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO domains (id, name, description, enabled, variables)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at, updated_at`,
		d.ID, d.Name, d.Description, d.Enabled, d.Variables,
	).Scan(&d.CreatedAt, &d.UpdatedAt)
	if isUniqueViolation(err) {
		return ErrAlreadyExists
	}
	return err
}

func (s *Store) GetDomain(ctx context.Context, name string) (*models.Domain, error) {
	var d models.Domain
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, COALESCE(description, ''), enabled, variables, created_at, updated_at
		FROM domains WHERE name = $1`, name,
	).Scan(&d.ID, &d.Name, &d.Description, &d.Enabled, &d.Variables, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *Store) ListDomains(ctx context.Context) ([]models.Domain, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, COALESCE(description, ''), enabled, variables, created_at, updated_at
		FROM domains ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Domain{}
	for rows.Next() {
		var d models.Domain
		if err := rows.Scan(&d.ID, &d.Name, &d.Description, &d.Enabled, &d.Variables, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) UpdateDomain(ctx context.Context, name string, d *models.Domain) error {
	if d.Variables == nil {
		d.Variables = map[string]string{}
	}
	err := s.pool.QueryRow(ctx, `
		UPDATE domains
		SET description = $2, enabled = $3, variables = $4, updated_at = NOW()
		WHERE name = $1
		RETURNING id, name, created_at, updated_at`,
		name, d.Description, d.Enabled, d.Variables,
	).Scan(&d.ID, &d.Name, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Store) DeleteDomain(ctx context.Context, name string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM domains WHERE name = $1`, name)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
