package store

import (
	"context"
	"errors"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// VoicemailStore reads mod_voicemail's message store from the freeswitch_core
// database (a SEPARATE pool from the control DB). It is almost entirely SELECTs;
// the one exception is MarkRead, which stamps read_epoch when a user listens to
// a message from the webphone (there is no FS API to do this for odbc storage).
type VoicemailStore struct {
	pool *pgxpool.Pool
}

func NewVoicemail(pool *pgxpool.Pool) *VoicemailStore {
	return &VoicemailStore{pool: pool}
}

// MessageFilePath returns the on-disk file_path of one message in a user's
// mailbox (matched by uuid), or "" if no such message. Used to stream the audio.
func (s *VoicemailStore) MessageFilePath(ctx context.Context, domain, number, uuid string) (string, error) {
	var fp string
	err := s.pool.QueryRow(ctx, `
		SELECT file_path FROM voicemail_msgs
		WHERE domain = $1 AND username = $2 AND uuid = $3`, domain, number, uuid).Scan(&fp)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return fp, nil
}

// MarkRead stamps read_epoch (and read_flags) on a still-unread message in a
// user's mailbox. Idempotent: a no-op if the message is missing or already read.
func (s *VoicemailStore) MarkRead(ctx context.Context, domain, number, uuid string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE voicemail_msgs
		SET read_epoch = EXTRACT(EPOCH FROM now())::int, read_flags = 'B_READ'
		WHERE domain = $1 AND username = $2 AND uuid = $3 AND read_epoch = 0`,
		domain, number, uuid)
	return err
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
