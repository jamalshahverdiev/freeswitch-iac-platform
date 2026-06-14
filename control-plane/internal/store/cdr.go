package store

import (
	"context"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
)

// InsertCDR stores a parsed CDR. Idempotent on the channel uuid (a retried
// POST from mod_json_cdr's failure queue won't duplicate).
func (s *Store) InsertCDR(ctx context.Context, c *models.CDR) error {
	if c.Raw == nil {
		c.Raw = []byte("{}")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO cdr (id, direction, caller_id_number, caller_id_name,
			destination_number, context, hangup_cause, start_epoch, answer_epoch,
			end_epoch, duration, billsec, recording_path, raw)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (id) DO NOTHING`,
		c.ID, c.Direction, c.CallerIDNumber, c.CallerIDName,
		c.DestinationNumber, c.Context, c.HangupCause, c.StartEpoch, c.AnswerEpoch,
		c.EndEpoch, c.Duration, c.Billsec, c.RecordingPath, c.Raw)
	return err
}

const cdrCols = `id, direction, caller_id_number, caller_id_name,
	destination_number, context, hangup_cause, start_epoch, answer_epoch,
	end_epoch, duration, billsec, recording_path, created_at`

// ListCDR returns CDRs newest-first per the filter, plus the unpaged total.
func (s *Store) ListCDR(ctx context.Context, f models.CDRFilter) ([]models.CDR, int, error) {
	where := `WHERE ($1='' OR caller_id_number=$1 OR destination_number=$1)
	            AND ($2='' OR hangup_cause=$2)
	            AND ($3=0 OR start_epoch >= $3)
	            AND ($4=0 OR start_epoch <= $4)
	            AND ($5=false OR billsec > 0)`
	args := []any{f.Number, f.HangupCause, f.FromEpoch, f.ToEpoch, f.AnsweredOnly}

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM cdr `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	q := `SELECT ` + cdrCols + ` FROM cdr ` + where + ` ORDER BY start_epoch DESC, id`
	if f.Limit > 0 {
		q += ` LIMIT $6 OFFSET $7`
		args = append(args, f.Limit, f.Offset)
	}
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []models.CDR{}
	for rows.Next() {
		var c models.CDR
		if err := rows.Scan(&c.ID, &c.Direction, &c.CallerIDNumber, &c.CallerIDName,
			&c.DestinationNumber, &c.Context, &c.HangupCause, &c.StartEpoch, &c.AnswerEpoch,
			&c.EndEpoch, &c.Duration, &c.Billsec, &c.RecordingPath, &c.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

// CDRStats returns per-day rollups (UTC) over the optional epoch window,
// newest day first.
func (s *Store) CDRStats(ctx context.Context, fromEpoch, toEpoch int64) ([]models.CDRStats, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT to_char(to_timestamp(start_epoch) AT TIME ZONE 'UTC', 'YYYY-MM-DD') AS day,
		       count(*)                                  AS total,
		       count(*) FILTER (WHERE billsec > 0)       AS answered,
		       count(*) FILTER (WHERE billsec = 0)       AS abandoned,
		       COALESCE(sum(billsec), 0)                 AS talk_time,
		       COALESCE(round(avg(billsec) FILTER (WHERE billsec > 0)), 0) AS avg_billsec
		FROM cdr
		WHERE ($1=0 OR start_epoch >= $1) AND ($2=0 OR start_epoch <= $2)
		GROUP BY day
		ORDER BY day DESC`, fromEpoch, toEpoch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.CDRStats{}
	for rows.Next() {
		var st models.CDRStats
		if err := rows.Scan(&st.Day, &st.Total, &st.Answered, &st.Abandoned,
			&st.TalkTime, &st.AvgBillsec); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}
