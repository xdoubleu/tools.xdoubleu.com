package models

import (
	"time"

	"github.com/google/uuid"
)

// OnlineWindow is how recently a paired runner must have been heard from to
// count as online: a little over two 20-second heartbeat holds.
const OnlineWindow = 45 * time.Second

// Runner is the daemon on the Mac that will run jobs. Its device token is
// stored only as a hash.
type Runner struct {
	ID        uuid.UUID
	Name      string
	CreatedBy string
	CreatedAt time.Time
	// PairingExpiresAt is set while a pairing code is waiting to be used.
	PairingExpiresAt *time.Time
	PairedAt         *time.Time
	LastSeen         *time.Time
	Version          string
	RevokedAt        *time.Time
}

// Online reports whether the runner is paired, active and recently seen.
func (r Runner) Online(now time.Time) bool {
	return r.PairedAt != nil && r.RevokedAt == nil && r.LastSeen != nil &&
		now.Sub(*r.LastSeen) < OnlineWindow
}
