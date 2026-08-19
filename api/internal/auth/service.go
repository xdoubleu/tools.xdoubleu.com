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

// SignInRenderFunc is called by TemplateAccess when the user is not authenticated.
// It receives the redirect URL so the sign-in page can redirect back after login.
type SignInRenderFunc func(w http.ResponseWriter, r *http.Request, redirectURL string)

// appUsersStore is the subset of *repositories.AppUsersRepository that
// LocalService needs. Narrowed to an interface (rather than the concrete
// type) so tests can fake individual failures — e.g. GetByID failing while
// Upsert succeeds — that aren't reachable by cancelling a context shared
// across both calls.
type appUsersStore interface {
	Upsert(ctx context.Context, id, email string) error
	GetByID(ctx context.Context, id string) (*models.User, error)
	GetAll(ctx context.Context) ([]models.User, error)
}

// OAuth2TokenResolver resolves a fosite-issued opaque OAuth 2.1 access token
// to the user ID it was granted for. Wired in by cmd/api after construction
// (like SignInRenderer) to avoid an import cycle between this package and
// internal/oauth2as.
type OAuth2TokenResolver interface {
	ResolveAccessToken(ctx context.Context, token string) (userID string, err error)
}

// claims are the JWT claims minted for a session access token.
type claims struct {
	jwt.RegisteredClaims
	AAL string `json:"aal"`
}

const (
	aal1 = "aal1"
	aal2 = "aal2"

	passwordResetTTL  = time.Hour
	refreshTokenBytes = 32
	recoveryCodeBytes = 10
)

// LocalService is the self-hosted bcrypt + TOTP + JWT implementation of
// Service (issue #1039), replacing the previous Supabase GoTrue-backed one.
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
	// resolveGroup coalesces concurrent resolveUser calls for the same
	// access token into a single verification, so opening several tabs at
	// once with the same (cache-miss) token doesn't repeat enrichment
	// queries per tab (see issue #852).
	resolveGroup singleflight.Group
	// SignInRenderer is set by cmd/api after construction to avoid a
	// circular import between this package and package main (which owns the
	// templ-generated SignInPage component).
	SignInRenderer SignInRenderFunc
	// OAuth2TokenResolver is set by cmd/api after construction (see the type
	// doc comment). Nil is valid: ResolveToken then only ever resolves
	// locally-minted session JWTs.
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

// InvalidateUserCache drops every cached user. Call it after mutations that
// change a user's role or app access so other sessions don't serve stale
// permissions for up to the cache TTL.
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

// generateOpaqueToken returns a URL-safe random token, hex-encoded, used for
// refresh tokens and password-reset tokens. Only its SHA-256 hash is stored.
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

// parseAccessToken verifies a session JWT and returns its claims.
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

// issueRefreshToken creates and stores a refresh token row for userID at the
// given aal, returning the opaque plaintext token.
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
	c, err := service.parseAccessToken(accessToken)
	if err != nil {
		return nil, err
	}

	row, err := service.usersStore.GetUserByID(ctx, c.Subject)
	if err != nil {
		return nil, err
	}

	_, hasMFA := service.HasVerifiedTOTP(ctx, accessToken)

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
		_ = service.usersStore.DeleteRefreshToken(ctx, row.ID)
		return nil, nil, errortools.NewUnauthorizedError(
			errors.New("refresh token expired"),
		)
	}

	// Rotate: the old token is single-use.
	if err = service.usersStore.DeleteRefreshToken(ctx, row.ID); err != nil {
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

// RefreshSession exchanges a refresh token for new tokens and returns the
// signed-in user plus ready-to-set access and refresh cookies. A nil user
// with nil error means the tokens rotated but the user lookup failed;
// callers should still set the cookies and treat the session as absent.
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

// SignOut deletes every refresh token belonging to the user resolved from
// accessToken. Ponytail: the Service interface only ever hands SignOut the
// access token (not the refresh-token cookie), and the simplest correct
// option for revoking server-side state on sign-out is to drop all of that
// user's refresh tokens rather than trying to single out the one presented
// alongside this access token — safe, since signing out of one session
// signing you out of all of them is expected behavior, not a bug.
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
		// Lax (not Strict): the web app's own /oauth/consent page redirects
		// the browser here cross-site during the MCP OAuth consent flow, and
		// a Strict cookie would not attach to that top-level GET. These
		// cookies don't gate cross-site CSRF on their own, so Lax is the
		// standard tradeoff.
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
		Secure:   secure,
		Path:     "/",
	}

	return &cookie, nil
}

// ForgotPassword generates and stores a password-reset token and emails a
// reset link. Errors resolving the user or sending the email are swallowed
// (mirroring the previous GoTrue-backed behavior) so this endpoint never
// reveals whether an email address has an account.
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
	accessToken, newPassword string,
) error {
	c, err := service.parseAccessToken(accessToken)
	if err != nil {
		return err
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

	// A password reset is meant to kick out any other session too.
	if err = service.usersStore.DeleteAllRefreshTokensForUser(
		ctx, c.Subject,
	); err != nil {
		return err
	}

	service.InvalidateUserCache()
	return nil
}

// ResetPasswordWithToken completes a forgot-password flow: validates the
// opaque reset token (hash + expiry + unused), sets the new password, marks
// the token used, and revokes every refresh token for that user.
func (service *LocalService) ResetPasswordWithToken(
	ctx context.Context,
	resetToken, newPassword string,
) error {
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
	if err = service.usersStore.MarkPasswordResetTokenUsed(ctx, row.ID); err != nil {
		return err
	}
	if err = service.usersStore.DeleteAllRefreshTokensForUser(
		ctx, row.UserID,
	); err != nil {
		return err
	}

	service.InvalidateUserCache()
	return nil
}
