package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/mail"
	"strings"
)

// Directory is the identity provider's user API (Zitadel by default).
type Directory interface {
	CreateHuman(ctx context.Context, email, displayName string) (string, error)
	SendInvite(ctx context.Context, userID string) error
	Deactivate(ctx context.Context, userID string) error
	Delete(ctx context.Context, userID string) error
}

var (
	// ErrDirectoryOff means no provider is configured to manage users at.
	ErrDirectoryOff = errors.New("auth: no identity provider is configured")
	// ErrInvalidEmail rejects an address that isn't a plain email.
	ErrInvalidEmail = errors.New("auth: invalid email address")
	// ErrUserExists rejects creating a second user with the same email.
	ErrUserExists = errors.New("auth: a user with this email already exists")
	// ErrUserNotFound means no such user.
	ErrUserNotFound = errors.New("auth: user not found")
)

// CreateUser adds a person at the provider, which mails the invite, and the
// linked user here. Every step is undone when a later one fails, so neither
// side keeps an orphan.
func (service *LocalService) CreateUser(
	ctx context.Context,
	email, displayName string,
) (*User, error) {
	if service.Directory == nil {
		return nil, ErrDirectoryOff
	}
	addr, err := mail.ParseAddress(strings.TrimSpace(email))
	if err != nil || addr.Name != "" {
		return nil, ErrInvalidEmail
	}
	email = strings.ToLower(addr.Address)
	displayName = strings.TrimSpace(displayName)

	_, err = service.usersStore.GetUserByEmail(ctx, email)
	if err == nil {
		return nil, ErrUserExists
	}
	if !errors.Is(err, errNotFound) {
		return nil, err
	}

	subject, err := service.Directory.CreateHuman(ctx, email, displayName)
	if err != nil {
		return nil, err
	}
	user, err := service.usersStore.CreateOIDCUser(ctx, email, subject)
	if err != nil {
		service.undo(ctx, subject, "", false)
		return nil, err
	}
	if err = service.finishCreate(ctx, user, displayName); err != nil {
		return nil, err
	}
	return user, nil
}

// finishCreate adds the app user and sends the invite, undoing the user on
// failure.
func (service *LocalService) finishCreate(
	ctx context.Context,
	user *User,
	displayName string,
) error {
	if service.appUsersRepo != nil {
		if err := service.appUsersRepo.Upsert(ctx, user.ID, user.Email); err != nil {
			service.undo(ctx, user.OIDCSubject, user.ID, false)
			return err
		}
		if displayName != "" {
			if err := service.appUsersRepo.SetDisplayName(
				ctx, user.ID, displayName,
			); err != nil {
				service.undo(ctx, user.OIDCSubject, user.ID, true)
				return err
			}
		}
	}
	if err := service.Directory.SendInvite(ctx, user.OIDCSubject); err != nil {
		service.undo(ctx, user.OIDCSubject, user.ID, true)
		return err
	}
	return nil
}

// undo removes what a failed create left behind: the provider's user, then the
// local rows. It uses a fresh context, as the request's may be the failure.
func (service *LocalService) undo(
	ctx context.Context,
	subject, userID string,
	appUser bool,
) {
	ctx = context.WithoutCancel(ctx)
	log := slog.Default()
	if appUser {
		if err := service.appUsersRepo.Delete(ctx, userID); err != nil {
			log.ErrorContext(ctx, "undo create: delete app user", "error", err)
		}
	}
	if userID != "" {
		if err := service.usersStore.DeleteUser(ctx, userID); err != nil {
			log.ErrorContext(ctx, "undo create: delete user", "error", err)
		}
	}
	if err := service.Directory.Delete(ctx, subject); err != nil {
		log.ErrorContext(ctx, "undo create: delete provider user",
			"subject", subject, "error", err)
	}
}

// DeactivateUser blocks the person at the provider, then here, and signs them
// out everywhere. The provider goes first so a failure leaves the user fully
// active and a retry safe.
func (service *LocalService) DeactivateUser(
	ctx context.Context,
	userID string,
) error {
	user, err := service.usersStore.GetUserByID(ctx, userID)
	if errors.Is(err, errNotFound) {
		return ErrUserNotFound
	}
	if err != nil {
		return err
	}
	if user.OIDCSubject != "" {
		if service.Directory == nil {
			return ErrDirectoryOff
		}
		if err = service.Directory.Deactivate(ctx, user.OIDCSubject); err != nil {
			return err
		}
	}
	if err = service.usersStore.SetDeactivated(ctx, userID); err != nil {
		return err
	}
	return service.revokeAllSessions(ctx, userID)
}
