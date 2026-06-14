package store

import (
	"context"
	"errors"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateDialplanExtension(ctx context.Context, e *models.DialplanExtension) error {
	domainID, err := s.domainID(ctx, e.Domain)
	if err != nil {
		return err
	}
	e.ID = uuid.NewString()
	if e.Priority == 0 {
		e.Priority = 100
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		INSERT INTO dialplan_extensions (id, domain_id, name, context, priority, enabled)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at`,
		e.ID, domainID, e.Name, e.Context, e.Priority, e.Enabled,
	).Scan(&e.CreatedAt, &e.UpdatedAt)
	if isUniqueViolation(err) {
		return ErrAlreadyExists
	}
	if err != nil {
		return err
	}

	if err := insertConditions(ctx, tx, e.ID, e.Conditions); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func insertConditions(ctx context.Context, tx pgx.Tx, extID string, conditions []models.DialplanCondition) error {
	for ci, c := range conditions {
		condID := uuid.NewString()
		timeAttrs := c.TimeAttrs
		if timeAttrs == nil {
			timeAttrs = map[string]string{}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO dialplan_conditions (id, extension_id, field, expression, time_attrs, order_index)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			condID, extID, c.Field, c.Expression, timeAttrs, ci); err != nil {
			return err
		}
		for ai, a := range c.Actions {
			if _, err := tx.Exec(ctx, `
				INSERT INTO dialplan_actions (id, condition_id, application, data, order_index)
				VALUES ($1, $2, $3, $4, $5)`,
				uuid.NewString(), condID, a.Application, a.Data, ai); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) loadConditions(ctx context.Context, extID string) ([]models.DialplanCondition, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, field, expression, time_attrs FROM dialplan_conditions
		WHERE extension_id = $1 ORDER BY order_index`, extID)
	if err != nil {
		return nil, err
	}
	type condRow struct {
		id   string
		cond models.DialplanCondition
	}
	var conds []condRow
	for rows.Next() {
		var cr condRow
		if err := rows.Scan(&cr.id, &cr.cond.Field, &cr.cond.Expression, &cr.cond.TimeAttrs); err != nil {
			rows.Close()
			return nil, err
		}
		if len(cr.cond.TimeAttrs) == 0 {
			cr.cond.TimeAttrs = nil // omit empty in JSON
		}
		conds = append(conds, cr)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]models.DialplanCondition, 0, len(conds))
	for _, cr := range conds {
		arows, err := s.pool.Query(ctx, `
			SELECT application, COALESCE(data,'') FROM dialplan_actions
			WHERE condition_id = $1 ORDER BY order_index`, cr.id)
		if err != nil {
			return nil, err
		}
		cr.cond.Actions = []models.DialplanAction{}
		for arows.Next() {
			var a models.DialplanAction
			if err := arows.Scan(&a.Application, &a.Data); err != nil {
				arows.Close()
				return nil, err
			}
			cr.cond.Actions = append(cr.cond.Actions, a)
		}
		arows.Close()
		if err := arows.Err(); err != nil {
			return nil, err
		}
		out = append(out, cr.cond)
	}
	return out, nil
}

func (s *Store) GetDialplanExtension(ctx context.Context, id string) (*models.DialplanExtension, error) {
	var e models.DialplanExtension
	err := s.pool.QueryRow(ctx, `
		SELECT e.id, COALESCE(d.name, ''), e.name, e.context, e.priority, e.enabled, e.created_at, e.updated_at
		FROM dialplan_extensions e LEFT JOIN domains d ON d.id = e.domain_id
		WHERE e.id = $1`, id,
	).Scan(&e.ID, &e.Domain, &e.Name, &e.Context, &e.Priority, &e.Enabled, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	e.Conditions, err = s.loadConditions(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (s *Store) ListDialplanExtensions(ctx context.Context, domainFilter, contextFilter string) ([]models.DialplanExtension, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, COALESCE(d.name,''), e.name, e.context, e.priority, e.enabled, e.created_at, e.updated_at
		FROM dialplan_extensions e LEFT JOIN domains d ON d.id = e.domain_id
		WHERE ($1 = '' OR d.name = $1) AND ($2 = '' OR e.context = $2)
		ORDER BY e.context, e.priority, e.name`, domainFilter, contextFilter)
	if err != nil {
		return nil, err
	}
	var ids []string
	exts := []models.DialplanExtension{}
	for rows.Next() {
		var e models.DialplanExtension
		if err := rows.Scan(&e.ID, &e.Domain, &e.Name, &e.Context, &e.Priority, &e.Enabled, &e.CreatedAt, &e.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		exts = append(exts, e)
		ids = append(ids, e.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range exts {
		conds, err := s.loadConditions(ctx, exts[i].ID)
		if err != nil {
			return nil, err
		}
		exts[i].Conditions = conds
	}
	return exts, nil
}

func (s *Store) UpdateDialplanExtension(ctx context.Context, id string, e *models.DialplanExtension) error {
	if e.Priority == 0 {
		e.Priority = 100
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		UPDATE dialplan_extensions
		SET name = $2, context = $3, priority = $4, enabled = $5, updated_at = NOW()
		WHERE id = $1
		RETURNING id, context, created_at, updated_at`,
		id, e.Name, e.Context, e.Priority, e.Enabled,
	).Scan(&e.ID, &e.Context, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM dialplan_conditions WHERE extension_id = $1`, id); err != nil {
		return err
	}
	if err := insertConditions(ctx, tx, id, e.Conditions); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) DeleteDialplanExtension(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM dialplan_extensions WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
