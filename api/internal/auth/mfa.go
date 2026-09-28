package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	qrcode "github.com/skip2/go-qrcode"
	"golang.org/x/crypto/bcrypt"

	"tools.xdoubleu.com/internal/errortools"
)

const (
	totpIssuer          = "tools.xdoubleu.com"
	totpQRSize          = 256
	totpSkew            = 1
	totpPeriodSeconds   = 30
	recoveryCodeCount   = 10
	recoveryCodeCharset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

// TOTPEnrollment is the result of beginning TOTP enrollment.
type TOTPEnrollment struct {
	ID     uuid.UUID
	Secret string
	QRSVG  string
}

// MFAChallenge is a synthetic ID that only preserves the two-step
// ChallengeMFA -> VerifyMFA call shape; TOTP itself needs no challenge.
type MFAChallenge struct {
	ID uuid.UUID
}

// HasVerifiedTOTP returns the verified TOTP factor ID for the token's user.
func (service *LocalService) HasVerifiedTOTP(
	ctx context.Context,
	accessToken string,
) (uuid.UUID, bool) {
	c, err := service.parseAccessToken(accessToken)
	if err != nil {
		return uuid.UUID{}, false
	}
	factor, err := service.usersStore.GetVerifiedTOTPFactor(ctx, c.Subject)
	if err != nil {
		return uuid.UUID{}, false
	}
	return factor.ID, true
}

// EnrollTOTP begins TOTP enrollment, deleting any unverified factor first.
func (service *LocalService) EnrollTOTP(
	ctx context.Context,
	accessToken string,
) (*TOTPEnrollment, error) {
	c, _, err := service.parseSessionToken(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	user, err := service.usersStore.GetUserByID(ctx, c.Subject)
	if err != nil {
		return nil, err
	}

	if err = service.usersStore.DeleteUnverifiedTOTPFactors(ctx, c.Subject); err != nil {
		return nil, err
	}

	//nolint:exhaustruct //other GenerateOpts fields use library defaults
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: user.Email,
	})
	if err != nil {
		return nil, err
	}

	sealed, err := service.sealer.Encrypt([]byte(key.Secret()))
	if err != nil {
		return nil, err
	}

	factorID, err := service.usersStore.CreateTOTPFactor(
		ctx, c.Subject, base64.StdEncoding.EncodeToString(sealed),
	)
	if err != nil {
		return nil, err
	}

	svg, err := renderQRSVG(key.URL())
	if err != nil {
		return nil, err
	}

	return &TOTPEnrollment{
		ID:     factorID,
		Secret: key.Secret(),
		QRSVG:  svg,
	}, nil
}

// renderQRSVG wraps a base64 PNG QR code in SVG; go-qrcode has no SVG output.
func renderQRSVG(otpURL string) (string, error) {
	png, err := qrcode.Encode(otpURL, qrcode.Medium, totpQRSize)
	if err != nil {
		return "", err
	}
	b64 := base64.StdEncoding.EncodeToString(png)
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">`+
			`<image width="%d" height="%d" href="data:image/png;base64,%s"/></svg>`,
		totpQRSize, totpQRSize, totpQRSize, totpQRSize, b64,
	), nil
}

// ChallengeMFA returns a fresh synthetic challenge ID (see MFAChallenge).
func (service *LocalService) ChallengeMFA(
	_ context.Context,
	_ string,
	_ uuid.UUID,
) (*MFAChallenge, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	return &MFAChallenge{ID: id}, nil
}

// VerifyMFA validates a TOTP code or unused recovery code, marks an
// unverified factor verified, and mints fresh aal2 tokens.
func (service *LocalService) VerifyMFA(
	ctx context.Context,
	accessToken string,
	factorID uuid.UUID,
	_ uuid.UUID,
	code string,
) (*string, *string, error) {
	invalidCode := errortools.NewUnauthorizedError(errors.New("invalid MFA code"))

	c, err := service.parseAccessToken(accessToken)
	if err != nil {
		return nil, nil, err
	}

	factor, err := service.usersStore.GetTOTPFactor(ctx, factorID)
	if err != nil || factor.UserID != c.Subject {
		return nil, nil, invalidCode
	}

	// Enrolling a second factor needs an aal2 session, not the pre-MFA token.
	if factor.Status != "verified" {
		if err = service.requireMFAIfEnrolled(ctx, c.Subject, c.AAL); err != nil {
			return nil, nil, err
		}
	}

	valid, err := service.checkFactorCode(ctx, factor, code)
	if err != nil {
		return nil, nil, err
	}
	if !valid {
		return nil, nil, invalidCode
	}

	if factor.Status == "unverified" {
		if err = service.usersStore.VerifyTOTPFactor(ctx, factorID); err != nil {
			return nil, nil, err
		}
	}

	service.userCache.evict(accessToken)

	newAccessToken, err := service.mintAccessToken(factor.UserID, aal2)
	if err != nil {
		return nil, nil, err
	}
	newRefreshToken, err := service.issueRefreshToken(ctx, factor.UserID, aal2)
	if err != nil {
		return nil, nil, err
	}

	return &newAccessToken, &newRefreshToken, nil
}

// VerifyCurrentFactor is the step-up check for factor management: code must
// be a current TOTP code or an unused recovery code of the user's factor.
func (service *LocalService) VerifyCurrentFactor(
	ctx context.Context,
	accessToken, code string,
) error {
	c, _, err := service.parseSessionToken(ctx, accessToken)
	if err != nil {
		return err
	}
	factor, err := service.usersStore.GetVerifiedTOTPFactor(ctx, c.Subject)
	if err != nil {
		return ErrWrongCredential
	}
	valid, err := service.checkFactorCode(ctx, factor, code)
	if err != nil {
		return err
	}
	if !valid {
		return ErrWrongCredential
	}
	return nil
}

// checkFactorCode accepts a TOTP code once per time step or, for a verified
// factor, an unused recovery code, which it consumes. Repeated wrong codes
// lock the user out (ErrTooManyAttempts).
func (service *LocalService) checkFactorCode(
	ctx context.Context,
	factor *TOTPFactor,
	code string,
) (bool, error) {
	attemptKey := "mfa:" + factor.UserID
	if err := service.attempts.check(attemptKey); err != nil {
		return false, err
	}
	valid, err := service.matchFactorCode(ctx, factor, code)
	if err != nil {
		return false, err
	}
	if valid {
		service.attempts.succeed(attemptKey)
	} else {
		service.attempts.fail(attemptKey)
	}
	return valid, nil
}

func (service *LocalService) matchFactorCode(
	ctx context.Context,
	factor *TOTPFactor,
	code string,
) (bool, error) {
	sealed, err := base64.StdEncoding.DecodeString(factor.Secret)
	if err != nil {
		return false, err
	}
	secret, err := service.sealer.Decrypt(sealed)
	if err != nil {
		return false, err
	}

	if step, ok := matchTOTPStep(string(secret), code, time.Now()); ok {
		return service.usersStore.ClaimTOTPStep(ctx, factor.ID, step)
	}
	if factor.Status == "verified" {
		return service.tryRecoveryCode(ctx, factor.UserID, code), nil
	}
	return false, nil
}

// matchTOTPStep returns the time step, within the allowed skew, whose code
// matches.
func matchTOTPStep(secret, code string, now time.Time) (int64, bool) {
	current := now.Unix() / totpPeriodSeconds
	for step := current - totpSkew; step <= current+totpSkew; step++ {
		//nolint:exhaustruct //Encoder uses the library default
		want, err := totp.GenerateCodeCustom(
			secret, time.Unix(step*totpPeriodSeconds, 0), totp.ValidateOpts{
				Period:    totpPeriodSeconds,
				Skew:      0,
				Digits:    otp.DigitsSix,
				Algorithm: otp.AlgorithmSHA1,
			},
		)
		if err == nil && subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			return step, true
		}
	}
	return 0, false
}

// tryRecoveryCode consumes the first unused recovery code matching code.
func (service *LocalService) tryRecoveryCode(
	ctx context.Context,
	userID, code string,
) bool {
	codes, err := service.usersStore.GetUnusedRecoveryCodes(ctx, userID)
	if err != nil {
		return false
	}
	for _, rc := range codes {
		if bcrypt.CompareHashAndPassword(
			[]byte(rc.CodeHash), []byte(code),
		) == nil {
			consumed, markErr := service.usersStore.MarkRecoveryCodeUsed(ctx, rc.ID)
			return markErr == nil && consumed
		}
	}
	return false
}

// UnenrollTOTP removes every TOTP factor and recovery code for the user.
func (service *LocalService) UnenrollTOTP(
	ctx context.Context,
	accessToken string,
	_ uuid.UUID,
) error {
	service.userCache.evict(accessToken)

	c, _, err := service.parseSessionToken(ctx, accessToken)
	if err != nil {
		return err
	}

	if err = service.usersStore.DeleteAllTOTPFactors(ctx, c.Subject); err != nil {
		return err
	}
	if err = service.usersStore.DeleteRecoveryCodes(ctx, c.Subject); err != nil {
		return err
	}
	return service.usersStore.RevokeOAuthGrants(ctx, c.Subject)
}

// GenerateRecoveryCodes replaces the user's recovery codes with 10 new ones,
// returning the plaintext once.
func (service *LocalService) GenerateRecoveryCodes(
	ctx context.Context,
	accessToken string,
) ([]string, error) {
	c, _, err := service.parseSessionToken(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	if err = service.usersStore.DeleteRecoveryCodes(ctx, c.Subject); err != nil {
		return nil, err
	}

	codes := make([]string, recoveryCodeCount)
	hashes := make([]string, recoveryCodeCount)
	for i := range codes {
		code, genErr := generateRecoveryCode()
		if genErr != nil {
			return nil, genErr
		}
		hash, hashErr := bcrypt.GenerateFromPassword(
			[]byte(code), bcrypt.DefaultCost,
		)
		if hashErr != nil {
			return nil, hashErr
		}
		codes[i] = code
		hashes[i] = string(hash)
	}

	if err = service.usersStore.CreateRecoveryCodes(ctx, c.Subject, hashes); err != nil {
		return nil, err
	}

	return codes, nil
}

func generateRecoveryCode() (string, error) {
	buf := make([]byte, recoveryCodeBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	out := make([]byte, recoveryCodeBytes)
	for i, b := range buf {
		out[i] = recoveryCodeCharset[int(b)%len(recoveryCodeCharset)]
	}
	return fmt.Sprintf("%s-%s", out[:5], out[5:]), nil
}
