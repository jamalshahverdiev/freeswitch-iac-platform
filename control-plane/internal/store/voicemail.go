package store

import (
	"context"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// VoicemailStore reads mod_voicemail's message store from the freeswitch_core
// database. It is a SEPARATE pool from the control DB: this data is written by
// FreeSWITCH, never by us, so we only ever issue SELECTs here.
type VoicemailStore struct {
	pool *pgxpool.Pool
}

func NewVoicemail(pool *pgxpool.Pool) *VoicemailStore {
	return &VoicemailStore{pool: pool}
}

// Messages returns a user's mailbox newest-first, with total/unread counters.
// A message is "unread" when read_epoch is 0 (FreeSWITCH stamps it on listen).
func (s *VoicemailStore) Messages(ctx context.Context, domain, number string) (*models.VoicemailBox, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT uuid, in_folder, cid_name, cid_number,
		       created_epoch, read_epoch, message_len
		FROM voicemail_msgs
		WHERE domain = $1 AND username = $2
		ORDER BY created_epoch DESC`, domain, number)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	box := &models.VoicemailBox{Domain: domain, Number: number, Messages: []models.VoicemailMessage{}}
	for rows.Next() {
		var m models.VoicemailMessage
		if err := rows.Scan(&m.UUID, &m.Folder, &m.CIDName, &m.CIDNumber,
			&m.CreatedEpoch, &m.ReadEpoch, &m.MessageLen); err != nil {
			return nil, err
		}
		m.Read = m.ReadEpoch > 0
		box.Total++
		if !m.Read {
			box.Unread++
		}
		box.Messages = append(box.Messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return box, nil
}
