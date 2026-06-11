package store

import (
	"context"
	"errors"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ---------- queues ----------

const ccQueueCols = `id, name, strategy, moh_sound, time_base_score,
	max_wait_time, max_wait_time_with_no_agent, max_wait_time_with_no_agent_time_reached,
	tier_rules_apply, tier_rule_wait_second, tier_rule_wait_multiply_level,
	tier_rule_no_agent_no_wait, discard_abandoned_after, abandoned_resume_allowed,
	params, created_at, updated_at`

func scanCCQueue(row pgx.Row, q *models.CCQueue) error {
	return row.Scan(&q.ID, &q.Name, &q.Strategy, &q.MohSound, &q.TimeBaseScore,
		&q.MaxWaitTime, &q.MaxWaitTimeWithNoAgent, &q.MaxWaitTimeWithNoAgentTimeReached,
		&q.TierRulesApply, &q.TierRuleWaitSecond, &q.TierRuleWaitMultiplyLevel,
		&q.TierRuleNoAgentNoWait, &q.DiscardAbandonedAfter, &q.AbandonedResumeAllowed,
		&q.Params, &q.CreatedAt, &q.UpdatedAt)
}

func (s *Store) CreateCCQueue(ctx context.Context, q *models.CCQueue) error {
	q.ID = uuid.NewString()
	if q.Params == nil {
		q.Params = map[string]string{}
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO cc_queues (id, name, strategy, moh_sound, time_base_score,
			max_wait_time, max_wait_time_with_no_agent, max_wait_time_with_no_agent_time_reached,
			tier_rules_apply, tier_rule_wait_second, tier_rule_wait_multiply_level,
			tier_rule_no_agent_no_wait, discard_abandoned_after, abandoned_resume_allowed, params)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		RETURNING created_at, updated_at`,
		q.ID, q.Name, q.Strategy, q.MohSound, q.TimeBaseScore,
		q.MaxWaitTime, q.MaxWaitTimeWithNoAgent, q.MaxWaitTimeWithNoAgentTimeReached,
		q.TierRulesApply, q.TierRuleWaitSecond, q.TierRuleWaitMultiplyLevel,
		q.TierRuleNoAgentNoWait, q.DiscardAbandonedAfter, q.AbandonedResumeAllowed, q.Params,
	).Scan(&q.CreatedAt, &q.UpdatedAt)
	if isUniqueViolation(err) {
		return ErrAlreadyExists
	}
	return err
}

func (s *Store) GetCCQueue(ctx context.Context, name string) (*models.CCQueue, error) {
	var q models.CCQueue
	err := scanCCQueue(s.pool.QueryRow(ctx,
		`SELECT `+ccQueueCols+` FROM cc_queues WHERE name = $1`, name), &q)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &q, nil
}

func (s *Store) ListCCQueues(ctx context.Context) ([]models.CCQueue, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+ccQueueCols+` FROM cc_queues ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.CCQueue{}
	for rows.Next() {
		var q models.CCQueue
		if err := scanCCQueue(rows, &q); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (s *Store) UpdateCCQueue(ctx context.Context, name string, q *models.CCQueue) error {
	if q.Params == nil {
		q.Params = map[string]string{}
	}
	err := s.pool.QueryRow(ctx, `
		UPDATE cc_queues SET strategy=$2, moh_sound=$3, time_base_score=$4,
			max_wait_time=$5, max_wait_time_with_no_agent=$6, max_wait_time_with_no_agent_time_reached=$7,
			tier_rules_apply=$8, tier_rule_wait_second=$9, tier_rule_wait_multiply_level=$10,
			tier_rule_no_agent_no_wait=$11, discard_abandoned_after=$12, abandoned_resume_allowed=$13,
			params=$14, updated_at=NOW()
		WHERE name=$1
		RETURNING id, name, created_at, updated_at`,
		name, q.Strategy, q.MohSound, q.TimeBaseScore,
		q.MaxWaitTime, q.MaxWaitTimeWithNoAgent, q.MaxWaitTimeWithNoAgentTimeReached,
		q.TierRulesApply, q.TierRuleWaitSecond, q.TierRuleWaitMultiplyLevel,
		q.TierRuleNoAgentNoWait, q.DiscardAbandonedAfter, q.AbandonedResumeAllowed, q.Params,
	).Scan(&q.ID, &q.Name, &q.CreatedAt, &q.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Store) DeleteCCQueue(ctx context.Context, name string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM cc_queues WHERE name=$1`, name)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- agents ----------

const ccAgentCols = `id, name, type, contact, status, max_no_answer, wrap_up_time,
	reject_delay_time, busy_delay_time, no_answer_delay_time, params, created_at, updated_at`

func scanCCAgent(row pgx.Row, a *models.CCAgent) error {
	return row.Scan(&a.ID, &a.Name, &a.Type, &a.Contact, &a.Status, &a.MaxNoAnswer,
		&a.WrapUpTime, &a.RejectDelayTime, &a.BusyDelayTime, &a.NoAnswerDelayTime,
		&a.Params, &a.CreatedAt, &a.UpdatedAt)
}

func (s *Store) CreateCCAgent(ctx context.Context, a *models.CCAgent) error {
	a.ID = uuid.NewString()
	if a.Params == nil {
		a.Params = map[string]string{}
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO cc_agents (id, name, type, contact, status, max_no_answer,
			wrap_up_time, reject_delay_time, busy_delay_time, no_answer_delay_time, params)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING created_at, updated_at`,
		a.ID, a.Name, a.Type, a.Contact, a.Status, a.MaxNoAnswer,
		a.WrapUpTime, a.RejectDelayTime, a.BusyDelayTime, a.NoAnswerDelayTime, a.Params,
	).Scan(&a.CreatedAt, &a.UpdatedAt)
	if isUniqueViolation(err) {
		return ErrAlreadyExists
	}
	return err
}

func (s *Store) GetCCAgent(ctx context.Context, name string) (*models.CCAgent, error) {
	var a models.CCAgent
	err := scanCCAgent(s.pool.QueryRow(ctx,
		`SELECT `+ccAgentCols+` FROM cc_agents WHERE name = $1`, name), &a)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) ListCCAgents(ctx context.Context) ([]models.CCAgent, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+ccAgentCols+` FROM cc_agents ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.CCAgent{}
	for rows.Next() {
		var a models.CCAgent
		if err := scanCCAgent(rows, &a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) UpdateCCAgent(ctx context.Context, name string, a *models.CCAgent) error {
	if a.Params == nil {
		a.Params = map[string]string{}
	}
	err := s.pool.QueryRow(ctx, `
		UPDATE cc_agents SET type=$2, contact=$3, status=$4, max_no_answer=$5,
			wrap_up_time=$6, reject_delay_time=$7, busy_delay_time=$8,
			no_answer_delay_time=$9, params=$10, updated_at=NOW()
		WHERE name=$1
		RETURNING id, name, created_at, updated_at`,
		name, a.Type, a.Contact, a.Status, a.MaxNoAnswer,
		a.WrapUpTime, a.RejectDelayTime, a.BusyDelayTime, a.NoAnswerDelayTime, a.Params,
	).Scan(&a.ID, &a.Name, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Store) DeleteCCAgent(ctx context.Context, name string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM cc_agents WHERE name=$1`, name)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- tiers ----------

func (s *Store) CreateCCTier(ctx context.Context, t *models.CCTier) error {
	t.ID = uuid.NewString()
	err := s.pool.QueryRow(ctx, `
		INSERT INTO cc_tiers (id, queue, agent, level, position)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING created_at, updated_at`,
		t.ID, t.Queue, t.Agent, t.Level, t.Position,
	).Scan(&t.CreatedAt, &t.UpdatedAt)
	if isUniqueViolation(err) {
		return ErrAlreadyExists
	}
	if isForeignKeyViolation(err) {
		return ErrNotFound
	}
	return err
}

func (s *Store) GetCCTier(ctx context.Context, queue, agent string) (*models.CCTier, error) {
	var t models.CCTier
	err := s.pool.QueryRow(ctx, `
		SELECT id, queue, agent, level, position, created_at, updated_at
		FROM cc_tiers WHERE queue=$1 AND agent=$2`, queue, agent,
	).Scan(&t.ID, &t.Queue, &t.Agent, &t.Level, &t.Position, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) ListCCTiers(ctx context.Context, queueFilter string) ([]models.CCTier, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, queue, agent, level, position, created_at, updated_at
		FROM cc_tiers WHERE ($1='' OR queue=$1) ORDER BY queue, level, position`, queueFilter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.CCTier{}
	for rows.Next() {
		var t models.CCTier
		if err := rows.Scan(&t.ID, &t.Queue, &t.Agent, &t.Level, &t.Position, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) UpdateCCTier(ctx context.Context, queue, agent string, t *models.CCTier) error {
	err := s.pool.QueryRow(ctx, `
		UPDATE cc_tiers SET level=$3, position=$4, updated_at=NOW()
		WHERE queue=$1 AND agent=$2
		RETURNING id, queue, agent, created_at, updated_at`,
		queue, agent, t.Level, t.Position,
	).Scan(&t.ID, &t.Queue, &t.Agent, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Store) DeleteCCTier(ctx context.Context, queue, agent string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM cc_tiers WHERE queue=$1 AND agent=$2`, queue, agent)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
