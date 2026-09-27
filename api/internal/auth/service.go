package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/xhit/go-str2duration/v2"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/singleflight"

	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/crypto"
	"tools.xdoubleu.com/internal/errortools"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/models"
)

// SignInRenderFunc renders the sign-in page for TemplateAccess.
type SignInRenderFunc func(w http.ResponseWriter, r *http.Request, redirectURL string)

// appUsersStore is an interface so tests can fake individual failures.
type appUsersStore interface {
	Upsert(ctx context.Context, id, email string) error
	GetByID(ctx context.Context, id string) (*models.User, error)
	GetAll(ctx context.Context) ([]models.User, error)
}

// OAuth2TokenResolver resolves a fosite opaque access token to a user ID. It is
// set by cmd/api to avoid an import cycle with internal/oauth2as.
type OAuth2TokenResolver interface {
	ResolveAccessToken(ctx context.Context, token string) (userID string, err error)
}

type claims struct {
	jwt.RegisteredClaims
	AAL string `json:"aal"`
}

const (
	aal1 = "aal1"
	aal2 = "aal2"

	passwordResetTTL  = time.Hour
	minPasswordLength = 8
	refreshTokenBytes = 32
	recoveryCodeBytes = 10
)

// LocalService is the self-hosted bcrypt + TOTP + JWT auth Service.
type LocalService struct {
	usersStore       usersStore
	jwtSecret        []byte
	sealer           *crypto.Sealer
	mailer           mailer.Client
	webURL           string
	useSecureCookies bool
	accessExpiry     string
	refreshExpiry    string
	appUsersRepo     appUsersStore
	userCache        *userCache
	// resolveGroup coalesces concurrent cache misses for the same token.
	resolveGroup singleflight.Group
	// SignInRenderer is set by cmd/api to avoid an import cycle with package main.
	SignInRenderer SignInRenderFunc
	// OAuth2TokenResolver may be nil: then only local session JWTs resolve.
	OAuth2TokenResolver OAuth2TokenResolver
}

var _ Service = (*LocalService)(nil)

func NewService(
	cfg config.Config,
	store usersStore,
	appUsersRepo appUsersStore,
	sealer *crypto.Sealer,
	mailerClient mailer.Client,
) *LocalService {
	// golang-jwt accepts an empty HMAC key, which would make sessions forgeable.
	if cfg.Env == config.ProdEnv && cfg.JWTSecret == "" {
		panic("auth: JWT_SECRET must be set in production")
	}
	return &LocalService{
		usersStore:       store,
		jwtSecret:        []byte(cfg.JWTSecret),
		sealer:           sealer,
		mailer:           mailerClient,
		webURL:           cfg.WebURL,
		useSecureCookies: cfg.Env == config.ProdEnv,
		accessExpiry:     cfg.AccessExpiry,
		refreshExpiry:    cfg.RefreshExpiry,
		appUsersRepo:     appUsersRepo,
		userCache: newUserCache(
			time.Duration(cfg.AuthCacheTTL) * time.Second,
		),
		resolveGroup:        singleflight.Group{},
		SignInRenderer:      nil,
		OAuth2TokenResolver: nil,
	}
}

// InvalidateUserCache drops every cached user; call it after role or
// app-access changes.
func (service *LocalService) InvalidateUserCache() {
	service.userCache.clear()
}

func (service *LocalService) GetAllUsers(
	ctx context.Context,
) ([]models.User, error) {
	if service.appUsersRepo != nil {
		return service.appUsersRepo.GetAll(ctx)
	}
	return []models.User{}, nil
}

// generateOpaqueToken returns a random hex token; store only its SHA-256.
func generateOpaqueToken(numBytes int) (string, error) {
	buf := make([]byte, numBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (service *LocalService) mintAccessToken(userID, aal string) (string, error) {
	ttl, err := str2duration.ParseDuration(service.accessExpiry)
	if err != nil {
		return "", err
	}
	now := time.Now()
	//nolint:exhaustruct //other RegisteredClaims fields are optional
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		AAL: aal,
	})
	return tok.SignedString(service.jwtSecret)
}

func (service *LocalService) parseAccessToken(accessToken string) (*claims, error) {
	var c claims
	token, err := jwt.ParseWithClaims(
		accessToken, &c,
		func(_ *jwt.Token) (any, error) { return service.jwtSecret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
	)
	if err != nil || !token.Valid {
		return nil, errortools.NewUnauthorizedError(errors.New("invalid token"))
	}
	return &c, nil
}

// ValidateAccessToken checks a session token's signature and expiry only, for
// callers that route an MFA user to the challenge themselves.
func (service *LocalService) ValidateAccessToken(accessToken string) error {
	_, err := service.parseAccessToken(accessToken)
	return err
}

//nolint:gochecknoglobals //computed once, read-only
var dummyPasswordHash = sync.OnceValue(func() []byte {
	hash, _ := bcrypt.GenerateFromPassword([]byte("unused"), bcrypt.DefaultCost)
	return hash
})

// ErrWrongCredential rejects a step-up check: a wrong current password, MFA
// code or recovery code.
var ErrWrongCredential = errors.New("current password or code is incorrect")

// ErrPasswordTooShort rejects a new password under minPasswordLength.
var ErrPasswordTooShort = fmt.Errorf(
	"password must be at least %d characters", minPasswordLength,
)

// errMFARequired rejects a pre-MFA (aal1) token of a user with a verified
// TOTP factor.
var errMFARequired = errortools.NewUnauthorizedError(errors.New("mfa required"))

// parseSessionToken parses a session access token, refusing an aal1 token once
// the user has a verified factor. It reports whether that factor exists.
func (service *LocalService) parseSessionToken(
	ctx context.Context,
	accessToken string,
) (*claims, bool, error) {
	c, err := service.parseAccessToken(accessToken)
	if err != nil {
		return nil, false, err
	}
	hasMFA, err := service.hasVerifiedFactor(ctx, c.Subject)
	if err != nil {
		return nil, false, err
	}
	if hasMFA && c.AAL != aal2 {
		return nil, false, errMFARequired
	}
	return c, hasMFA, nil
}

// requireMFAIfEnrolled refuses a non-aal2 credential once userID has a
// verified factor.
func (service *LocalService) requireMFAIfEnrolled(
	ctx context.Context,
	userID, aal string,
) error {
	if aal == aal2 {
		return nil
	}
	hasMFA, err := service.hasVerifiedFactor(ctx, userID)
	if err != nil {
		return err
	}
	if hasMFA {
		return errMFARequired
	}
	return nil
}

// hasVerifiedFactor fails closed: only a not-found result means no factor.
func (service *LocalService) hasVerifiedFactor(
	ctx context.Context,
	userID string,
) (bool, error) {
	_, err := service.usersStore.GetVerifiedTOTPFactor(ctx, userID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, errNotFound) {
		return false, nil
	}
	return false, err
}

func (service *LocalService) issueRefreshToken(
	ctx context.Context,
	userID, aal string,
) (string, error) {
	ttl, err := str2duration.ParseDuration(service.refreshExpiry)
	if err != nil {
		return "", err
	}
	token, err := generateOpaqueToken(refreshTokenBytes)
	if err != nil {
		return "", err
	}
	if err = service.usersStore.CreateRefreshToken(
		ctx, userID, hashToken(token), aal, time.Now().Add(ttl),
	); err != nil {
		return "", err
	}
	return token, nil
}

func (service *LocalService) SignInWithEmail(
	ctx context.Context,
	email, password string,
) (*string, *string, error) {
	invalidCreds := errortools.NewUnauthorizedError(errors.New("invalid credentials"))

	user, err := service.usersStore.GetUserByEmail(ctx, email)
	if err != nil {
		// Same bcrypt cost as a real account, so timing doesn't reveal which
		// emails are registered.
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash(), []byte(password))
		return nil, nil, invalidCreds
	}

	if bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash), []byte(password),
	) != nil {
		return nil, nil, invalidCreds
	}

	accessToken, err := service.mintAccessToken(user.ID, aal1)
	if err != nil {
		return nil, nil, err
	}
	refreshToken, err := service.issueRefreshToken(ctx, user.ID, aal1)
	if err != nil {
		return nil, nil, err
	}

	return &accessToken, &refreshToken, nil
}

func (service *LocalService) GetUser(
	ctx context.Context,
	accessToken string,
) (*models.User, error) {
	c, hasMFA, err := service.parseSessionToken(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	row, err := service.usersStore.GetUserByID(ctx, c.Subject)
	if err != nil {
		return nil, err
	}

	return &models.User{
		ID:          row.ID,
		Email:       row.Email,
		Role:        models.RoleUser,
		AppAccess:   []string{},
		HasMFA:      hasMFA,
		DisplayName: "",
	}, nil
}

func (service *LocalService) SignInWithRefreshToken(
	ctx context.Context,
	refreshToken string,
) (*string, *string, error) {
	row, err := service.usersStore.GetRefreshTokenByHash(ctx, hashToken(refreshToken))
	if err != nil {
		return nil, nil, errortools.NewUnauthorizedError(
			errors.New("invalid refresh token"),
		)
	}
	if time.Now().After(row.ExpiresAt) {
		_, _ = service.usersStore.DeleteRefreshToken(ctx, row.ID)
		return nil, nil, errortools.NewUnauthorizedError(
			errors.New("refresh token expired"),
		)
	}

	// Rotate: the old token is single-use, so only the caller that deletes it
	// gets new tokens.
	deleted, err := service.usersStore.DeleteRefreshToken(ctx, row.ID)
	if err != nil {
		return nil, nil, err
	}
	if !deleted {
		return nil, nil, errortools.NewUnauthorizedError(
			errors.New("refresh token already used"),
		)
	}

	if err = service.requireMFAIfEnrolled(ctx, row.UserID, row.AAL); err != nil {
		return nil, nil, err
	}

	newRefreshToken, err := service.issueRefreshToken(ctx, row.UserID, row.AAL)
	if err != nil {
		return nil, nil, err
	}

	accessToken, err := service.mintAccessToken(row.UserID, row.AAL)
	if err != nil {
		return nil, nil, err
	}

	return &accessToken, &newRefreshToken, nil
}

// RefreshSession rotates tokens and returns the user plus cookies. A nil user
// with nil error means rotation succeeded but the user lookup failed.
func (service *LocalService) RefreshSession(
	ctx context.Context,
	refreshToken string,
) (*models.User, *http.Cookie, *http.Cookie, error) {
	accessToken, newRefreshToken, err := service.SignInWithRefreshToken(
		ctx,
		refreshToken,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	accessCookie, err := service.CreateCookie(
		models.AccessScope,
		*accessToken,
		service.accessExpiry,
		service.useSecureCookies,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	refreshCookie, err := service.CreateCookie(
		models.RefreshScope,
		*newRefreshToken,
		service.refreshExpiry,
		service.useSecureCookies,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	user, _ := service.GetUser(ctx, *accessToken)
	return user, accessCookie, refreshCookie, nil
}

// SignOut deletes all of the user's refresh tokens (it only receives the
// access token), signing out every session.
func (service *LocalService) SignOut(
	ctx context.Context,
	accessToken string,
	secure bool,
) (*http.Cookie, *http.Cookie, error) {
	service.userCache.evict(accessToken)

	if c, err := service.parseAccessToken(accessToken); err == nil {
		_ = service.usersStore.DeleteAllRefreshTokensForUser(ctx, c.Subject)
	}

	//nolint:gosec // Secure is conditionally set based on environment
	deleteAccessTokenCookie := &http.Cookie{
		Name:     service.GetCookieName(models.AccessScope),
		Value:    "",
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
		Secure:   secure,
		Path:     "/",
	}

	//nolint:gosec // Secure is conditionally set based on environment
	deleteRefreshTokenCookie := &http.Cookie{
		Name:     service.GetCookieName(models.RefreshScope),
		Value:    "",
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
		Secure:   secure,
		Path:     "/",
	}

	return deleteAccessTokenCookie, deleteRefreshTokenCookie, nil
}

func (service *LocalService) GetCookieName(scope models.Scope) string {
	switch scope {
	case models.AccessScope:
		return "accessToken"
	case models.RefreshScope:
		return "refreshToken"
	default:
		panic("invalid scope")
	}
}

func (service *LocalService) CreateCookie(
	scope models.Scope,
	token string,
	expiry string,
	secure bool,
) (*http.Cookie, error) {
	ttl, err := str2duration.ParseDuration(expiry)
	if err != nil {
		return nil, err
	}

	name := service.GetCookieName(scope)

	//nolint:gosec // Secure is conditionally set based on environment
	cookie := http.Cookie{
		Name:    name,
		Value:   token,
		Expires: time.Now().Add(ttl),
		// Lax, not Strict: the MCP OAuth consent flow redirects here cross-site.
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
		Secure:   secure,
		Path:     "/",
	}

	return &cookie, nil
}

// ForgotPassword emails a reset link. Errors are swallowed so the endpoint
// never reveals whether an email has an account.
func (service *LocalService) ForgotPassword(
	ctx context.Context,
	email, redirectTo string,
) error {
	user, err := service.usersStore.GetUserByEmail(ctx, email)
	if err != nil {
		return nil //nolint:nilerr //deliberately not revealing account existence
	}

	token, err := generateOpaqueToken(refreshTokenBytes)
	if err != nil {
		return nil //nolint:nilerr //see above
	}

	if err = service.usersStore.CreatePasswordResetToken(
		ctx, user.ID, hashToken(token), time.Now().Add(passwordResetTTL),
	); err != nil {
		return nil //nolint:nilerr //see above
	}

	link := fmt.Sprintf("%s?token=%s", redirectTo, token)
	if service.mailer != nil {
		_ = service.mailer.SendTo(
			ctx, email, "Reset your password",
			fmt.Sprintf("Reset your password: %s", link),
		)
	}
	return nil
}

func (service *LocalService) UpdatePassword(
	ctx context.Context,
	accessToken, currentPassword, newPassword string,
) error {
	if len(newPassword) < minPasswordLength {
		return ErrPasswordTooShort
	}

	c, _, err := service.parseSessionToken(ctx, accessToken)
	if err != nil {
		return err
	}

	user, err := service.usersStore.GetUserByID(ctx, c.Subject)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash), []byte(currentPassword),
	) != nil {
		return ErrWrongCredential
	}

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(newPassword), bcrypt.DefaultCost,
	)
	if err != nil {
		return err
	}

	if err = service.usersStore.SetPasswordHash(ctx, c.Subject, string(hash)); err != nil {
		return err
	}

	return service.revokeAllSessions(ctx, c.Subject)
}

// revokeAllSessions signs the user out everywhere: every refresh token and
// every OAuth grant.
func (service *LocalService) revokeAllSessions(
	ctx context.Context,
	userID string,
) error {
	if err := service.usersStore.DeleteAllRefreshTokensForUser(
		ctx, userID,
	); err != nil {
		return err
	}
	if err := service.usersStore.RevokeOAuthGrants(ctx, userID); err != nil {
		return err
	}
	service.InvalidateUserCache()
	return nil
}

// ResetPasswordWithToken consumes the reset token, sets the password and
// signs the user out everywhere.
func (service *LocalService) ResetPasswordWithToken(
	ctx context.Context,
	resetToken, newPassword string,
) error {
	if len(newPassword) < minPasswordLength {
		return ErrPasswordTooShort
	}

	row, err := service.usersStore.GetPasswordResetTokenByHash(
		ctx, hashToken(resetToken),
	)
	if err != nil {
		return errortools.NewUnauthorizedError(errors.New("invalid reset token"))
	}
	if row.UsedAt != nil {
		return errortools.NewUnauthorizedError(errors.New("reset token already used"))
	}
	if time.Now().After(row.ExpiresAt) {
		return errortools.NewUnauthorizedError(errors.New("reset token expired"))
	}

	// Consume first, so concurrent uses of one token can't both reset.
	consumed, err := service.usersStore.MarkPasswordResetTokenUsed(ctx, row.ID)
	if err != nil {
		return err
	}
	if !consumed {
		return errortools.NewUnauthorizedError(errors.New("reset token already used"))
	}

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(newPassword), bcrypt.DefaultCost,
	)
	if err != nil {
		return err
	}

	if err = service.usersStore.SetPasswordHash(
		ctx, row.UserID, string(hash),
	); err != nil {
		return err
	}

	return service.revokeAllSessions(ctx, row.UserID)
}
