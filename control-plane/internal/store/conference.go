package store

import (
	"context"
	"errors"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ---------- profiles ----------

const confProfileCols = `id, name, rate, interval_ms, energy_level, comfort_noise,
	moh_sound, video_mode, video_layout, video_canvas_size, video_fps, auto_record,
	params, created_at, updated_at`

func scanConfProfile(row pgx.Row, p *models.ConferenceProfile) error {
	return row.Scan(&p.ID, &p.Name, &p.Rate, &p.IntervalMs, &p.EnergyLevel, &p.ComfortNoise,
		&p.MohSound, &p.VideoMode, &p.VideoLayout, &p.VideoCanvasSize, &p.VideoFPS, &p.AutoRecord,
		&p.Params, &p.CreatedAt, &p.UpdatedAt)
}

func (s *Store) CreateConferenceProfile(ctx context.Context, p *models.ConferenceProfile) error {
	p.ID = uuid.NewString()
	if p.Params == nil {
		p.Params = map[string]string{}
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO conference_profiles (id, name, rate, interval_ms, energy_level, comfort_noise,
			moh_sound, video_mode, video_layout, video_canvas_size, video_fps, auto_record, params)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING created_at, updated_at`,
		p.ID, p.Name, p.Rate, p.IntervalMs, p.EnergyLevel, p.ComfortNoise,
		p.MohSound, p.VideoMode, p.VideoLayout, p.VideoCanvasSize, p.VideoFPS, p.AutoRecord, p.Params,
	).Scan(&p.CreatedAt, &p.UpdatedAt)
	if isUniqueViolation(err) {
		return ErrAlreadyExists
	}
	return err
}

func (s *Store) GetConferenceProfile(ctx context.Context, name string) (*models.ConferenceProfile, error) {
	var p models.ConferenceProfile
	err := scanConfProfile(s.pool.QueryRow(ctx,
		`SELECT `+confProfileCols+` FROM conference_profiles WHERE name = $1`, name), &p)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) ListConferenceProfiles(ctx context.Context) ([]models.ConferenceProfile, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+confProfileCols+` FROM conference_profiles ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.ConferenceProfile{}
	for rows.Next() {
		var p models.ConferenceProfile
		if err := scanConfProfile(rows, &p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) UpdateConferenceProfile(ctx context.Context, name string, p *models.ConferenceProfile) error {
	if p.Params == nil {
		p.Params = map[string]string{}
	}
	err := s.pool.QueryRow(ctx, `
		UPDATE conference_profiles SET rate=$2, interval_ms=$3, energy_level=$4, comfort_noise=$5,
			moh_sound=$6, video_mode=$7, video_layout=$8, video_canvas_size=$9, video_fps=$10,
			auto_record=$11, params=$12, updated_at=NOW()
		WHERE name=$1
		RETURNING id, name, created_at, updated_at`,
		name, p.Rate, p.IntervalMs, p.EnergyLevel, p.ComfortNoise,
		p.MohSound, p.VideoMode, p.VideoLayout, p.VideoCanvasSize, p.VideoFPS,
		p.AutoRecord, p.Params,
	).Scan(&p.ID, &p.Name, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Store) DeleteConferenceProfile(ctx context.Context, name string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM conference_profiles WHERE name=$1`, name)
	if isForeignKeyViolation(err) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- rooms ----------

const confRoomCols = `id, name, number, domain, context, profile, pin, max_members,
	priority, enabled, created_at, updated_at`

func scanConfRoom(row pgx.Row, r *models.ConferenceRoom) error {
	return row.Scan(&r.ID, &r.Name, &r.Number, &r.Domain, &r.Context, &r.Profile, &r.Pin,
		&r.MaxMembers, &r.Priority, &r.Enabled, &r.CreatedAt, &r.UpdatedAt)
}

func (s *Store) CreateConferenceRoom(ctx context.Context, r *models.ConferenceRoom) error {
	r.ID = uuid.NewString()
	err := s.pool.QueryRow(ctx, `
		INSERT INTO conference_rooms (id, name, number, domain, context, profile, pin,
			max_members, priority, enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING created_at, updated_at`,
		r.ID, r.Name, r.Number, r.Domain, r.Context, r.Profile, r.Pin,
		r.MaxMembers, r.Priority, r.Enabled,
	).Scan(&r.CreatedAt, &r.UpdatedAt)
	if isUniqueViolation(err) {
		return ErrAlreadyExists
	}
	if isForeignKeyViolation(err) {
		return ErrNotFound
	}
	return err
}

func (s *Store) GetConferenceRoom(ctx context.Context, name string) (*models.ConferenceRoom, error) {
	var r models.ConferenceRoom
	err := scanConfRoom(s.pool.QueryRow(ctx,
		`SELECT `+confRoomCols+` FROM conference_rooms WHERE name = $1`, name), &r)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) ListConferenceRooms(ctx context.Context, contextFilter string) ([]models.ConferenceRoom, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+confRoomCols+` FROM conference_rooms
		WHERE ($1='' OR context=$1) ORDER BY priority, name`, contextFilter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.ConferenceRoom{}
	for rows.Next() {
		var r models.ConferenceRoom
		if err := scanConfRoom(rows, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) UpdateConferenceRoom(ctx context.Context, name string, r *models.ConferenceRoom) error {
	err := s.pool.QueryRow(ctx, `
		UPDATE conference_rooms SET number=$2, domain=$3, context=$4, profile=$5, pin=$6,
			max_members=$7, priority=$8, enabled=$9, updated_at=NOW()
		WHERE name=$1
		RETURNING id, name, created_at, updated_at`,
		name, r.Number, r.Domain, r.Context, r.Profile, r.Pin,
		r.MaxMembers, r.Priority, r.Enabled,
	).Scan(&r.ID, &r.Name, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if isForeignKeyViolation(err) {
		return ErrNotFound
	}
	return err
}

func (s *Store) DeleteConferenceRoom(ctx context.Context, name string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM conference_rooms WHERE name=$1`, name)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
