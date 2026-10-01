package main

import (
	"context"
	"errors"
	"fmt"

	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/oauthconn"
)

const securityEventsScope = "security_events"

// securityAlertsError explains a failed fetch. On a denial it names
// security_events when the connection's granted scope doesn't list it.
func (h *obsConnectHandler) securityAlertsError(
	ctx context.Context, err error,
) string {
	if !errors.Is(err, github.ErrAccessDenied) {
		return fmt.Sprintf("security alerts unavailable: %v", err)
	}

	_, conn, connErr := h.app.oauthConnRepo.Get(ctx, models.OAuthProviderGithub)
	if connErr == nil &&
		!oauthconn.HasScopes(conn.GrantedScope, []string{securityEventsScope}) {
		return fmt.Sprintf(
			"GitHub denied access to security alerts: the connection's granted "+
				"scope %q lacks %s; reconnect GitHub on /monitoring/connections "+
				"to grant it (%v)",
			conn.GrantedScope, securityEventsScope, err,
		)
	}
	return fmt.Sprintf("GitHub denied access to security alerts: %v", err)
}
