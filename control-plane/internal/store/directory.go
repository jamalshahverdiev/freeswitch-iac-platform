package store

import (
	"context"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
)

// DirectoryData returns enabled domains together with their enabled users,
// ordered deterministically for stable XML rendering.
func (s *Store) DirectoryData(ctx context.Context) ([]models.DomainWithUsers, error) {
	domains, err := s.ListDomains(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]models.DomainWithUsers, 0, len(domains))
	for _, d := range domains {
		if !d.Enabled {
			continue
		}
		users, err := s.ListUsers(ctx, d.Name)
		if err != nil {
			return nil, err
		}
		enabled := make([]models.User, 0, len(users))
		for _, u := range users {
			if u.Enabled {
				enabled = append(enabled, u)
			}
		}
		out = append(out, models.DomainWithUsers{Domain: d, Users: enabled})
	}
	return out, nil
}
