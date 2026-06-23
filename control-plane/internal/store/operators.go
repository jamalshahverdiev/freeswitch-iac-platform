package store

import (
	"context"
	"errors"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const operatorCols = `id, subject, domain, number, display_name, enabled, created_at, updated_at`

func scanOperator(row pgx.Row, o *models.Operator) error {
	return row.Scan(&o.ID, &o.Subject, &o.Domain, &o.Number, &o.DisplayName,
		&o.Enabled, &o.CreatedAt, &o.UpdatedAt)
}

func (s *Store) CreateOperator(ctx context.Context, o *models.Operator) error {
	o.ID = uuid.NewString()
	err := s.pool.QueryRow(ctx, `
		INSERT INTO operators (id, subject, domain, number, display_name, enabled)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING created_at, updated_at`,
		o.ID, o.Subject, o.Domain, o.Number, o.DisplayName, o.Enabled,
	).Scan(&o.CreatedAt, &o.UpdatedAt)
	if isUniqueViolation(err) {
		return ErrAlreadyExists
	}
	return err
}

func (s *Store) GetOperator(ctx context.Context, subject string) (*models.Operator, error) {
	var o models.Operator
	err := scanOperator(s.pool.QueryRow(ctx,
		`SELECT `+operatorCols+` FROM operators WHERE subject = $1`, subject), &o)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (s *Store) ListOperators(ctx context.Context) ([]models.Operator, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+operatorCols+` FROM operators ORDER BY subject`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Operator{}
	for rows.Next() {
		var o models.Operator
		if err := scanOperator(rows, &o); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Store) UpdateOperator(ctx context.Context, subject string, o *models.Operator) error {
	err := s.pool.QueryRow(ctx, `
		UPDATE operators SET domain=$2, number=$3, display_name=$4, enabled=$5, updated_at=NOW()
		WHERE subject=$1
		RETURNING id, subject, created_at, updated_at`,
		subject, o.Domain, o.Number, o.DisplayName, o.Enabled,
	).Scan(&o.ID, &o.Subject, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Store) DeleteOperator(ctx context.Context, subject string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM operators WHERE subject=$1`, subject)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
