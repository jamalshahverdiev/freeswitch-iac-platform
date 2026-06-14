package store

import (
	"context"
	"errors"
	"strings"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// NormalizeMAC lowercases a MAC and strips separators (":", "-", ".").
func NormalizeMAC(mac string) string {
	r := strings.NewReplacer(":", "", "-", "", ".", "", " ", "")
	return strings.ToLower(r.Replace(mac))
}

const deviceCols = `id, mac, vendor, model, number, domain, display_name, enabled, created_at, updated_at`

func scanDevice(row pgx.Row, d *models.Device) error {
	return row.Scan(&d.ID, &d.MAC, &d.Vendor, &d.Model, &d.Number, &d.Domain,
		&d.DisplayName, &d.Enabled, &d.CreatedAt, &d.UpdatedAt)
}

func (s *Store) CreateDevice(ctx context.Context, d *models.Device) error {
	d.ID = uuid.NewString()
	d.MAC = NormalizeMAC(d.MAC)
	err := s.pool.QueryRow(ctx, `
		INSERT INTO provisioned_devices (id, mac, vendor, model, number, domain, display_name, enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING created_at, updated_at`,
		d.ID, d.MAC, d.Vendor, d.Model, d.Number, d.Domain, d.DisplayName, d.Enabled,
	).Scan(&d.CreatedAt, &d.UpdatedAt)
	if isUniqueViolation(err) {
		return ErrAlreadyExists
	}
	return err
}

func (s *Store) GetDevice(ctx context.Context, mac string) (*models.Device, error) {
	var d models.Device
	err := scanDevice(s.pool.QueryRow(ctx,
		`SELECT `+deviceCols+` FROM provisioned_devices WHERE mac = $1`, NormalizeMAC(mac)), &d)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *Store) ListDevices(ctx context.Context) ([]models.Device, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+deviceCols+` FROM provisioned_devices ORDER BY mac`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Device{}
	for rows.Next() {
		var d models.Device
		if err := scanDevice(rows, &d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) UpdateDevice(ctx context.Context, mac string, d *models.Device) error {
	err := s.pool.QueryRow(ctx, `
		UPDATE provisioned_devices SET vendor=$2, model=$3, number=$4, domain=$5,
			display_name=$6, enabled=$7, updated_at=NOW()
		WHERE mac=$1
		RETURNING id, mac, created_at, updated_at`,
		NormalizeMAC(mac), d.Vendor, d.Model, d.Number, d.Domain, d.DisplayName, d.Enabled,
	).Scan(&d.ID, &d.MAC, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Store) DeleteDevice(ctx context.Context, mac string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM provisioned_devices WHERE mac=$1`, NormalizeMAC(mac))
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeviceAccount resolves a device by MAC and joins the SIP password from its
// directory user (params->>'password'). ErrNotFound if the device is missing,
// disabled, or its user/password is absent.
func (s *Store) DeviceAccount(ctx context.Context, mac string) (*models.DeviceAccount, error) {
	d, err := s.GetDevice(ctx, mac)
	if err != nil {
		return nil, err
	}
	if !d.Enabled {
		return nil, ErrNotFound
	}
	var password string
	err = s.pool.QueryRow(ctx, `
		SELECT COALESCE(u.params->>'password','')
		FROM users u JOIN domains dom ON dom.id = u.domain_id
		WHERE dom.name = $1 AND u.number = $2`, d.Domain, d.Number).Scan(&password)
	if errors.Is(err, pgx.ErrNoRows) || password == "" {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &models.DeviceAccount{Device: *d, Password: password}, nil
}
