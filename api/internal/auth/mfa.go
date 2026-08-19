package auth

import (
	"context"
	"crypto/rand"
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

// MFAChallenge stands in for the challenge concept GoTrue exposed
// server-side. pquerna/otp has no equivalent — TOTP verification only needs
// the code and the stored secret — so this is an intentional simplification:
// a fresh synthetic ID is generated purely to preserve the existing two-step
// ChallengeMFA -> VerifyMFA(challengeID) call shape the Connect handlers use.
type MFAChallenge struct {
	ID uuid.UUID
}

// HasVerifiedTOTP returns the factor ID of the verified TOTP factor for the
// user resolved from accessToken, or (zero, false) if none / token invalid.
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

// EnrollTOTP begins TOTP enrollment for the user resolved from accessToken
// and returns the QR code (SVG-wrapped PNG), fallback secret, and factor ID.
// Any pre-existing unverified TOTP factor is deleted first so a repeated
// enrollment attempt doesn't leave stale rows behind.
func (service *LocalService) EnrollTOTP(
	ctx context.Context,
	accessToken string,
) (*TOTPEnrollment, error) {
	c, err := service.parseAccessToken(accessToken)
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

// renderQRSVG renders a QR code for otpURL as an SVG wrapping a base64 PNG —
// go-qrcode has no native SVG output, and this keeps the return type the
// callers (an <img>-shaped QrSvg proto field) already expect.
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

// ChallengeMFA returns a fresh synthetic challenge ID — see MFAChallenge's
// doc comment for why this is a no-op beyond that.
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

// VerifyMFA validates a TOTP code (or, failing that, an unused recovery
// code) against the given factor. It also completes enrollment: an
// unverified factor is marked verified on the first successful code. On
// success it evicts the cache and mints fresh aal2 access and refresh
// tokens.
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

	sealed, err := base64.StdEncoding.DecodeString(factor.Secret)
	if err != nil {
		return nil, nil, err
	}
	secret, err := service.sealer.Decrypt(sealed)
	if err != nil {
		return nil, nil, err
	}

	// A malformed code (e.g. a recovery code's "XXXXX-XXXXX" shape rather
	// than 6 digits) makes ValidateCustom return an error — treated the same
	// as an invalid TOTP code here, not fatal, so the recovery-code fallback
	// below still gets a chance to run.
	//nolint:exhaustruct //Encoder uses the library default
	valid, _ := totp.ValidateCustom(
		code, string(secret), time.Now(), totp.ValidateOpts{
			Period:    totpPeriodSeconds,
			Skew:      totpSkew,
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		},
	)

	if !valid && factor.Status == "verified" {
		valid = service.tryRecoveryCode(ctx, factor.UserID, code)
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

// tryRecoveryCode checks code against every unused recovery code for
// userID, consuming (marking used) the first match.
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
			_ = service.usersStore.MarkRecoveryCodeUsed(ctx, rc.ID)
			return true
		}
	}
	return false
}

// UnenrollTOTP removes every TOTP factor and recovery code for the user
// resolved from accessToken.
func (service *LocalService) UnenrollTOTP(
	ctx context.Context,
	accessToken string,
	_ uuid.UUID,
) error {
	service.userCache.evict(accessToken)

	c, err := service.parseAccessToken(accessToken)
	if err != nil {
		return err
	}

	if err = service.usersStore.DeleteAllTOTPFactors(ctx, c.Subject); err != nil {
		return err
	}
	return service.usersStore.DeleteRecoveryCodes(ctx, c.Subject)
}

// GenerateRecoveryCodes replaces the user's recovery codes with a fresh set
// of 10 and returns their plaintext (only ever available once).
func (service *LocalService) GenerateRecoveryCodes(
	ctx context.Context,
	accessToken string,
) ([]string, error) {
	c, err := service.parseAccessToken(accessToken)
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

// generateRecoveryCode returns a 10-character alphanumeric code grouped as
// "XXXXX-XXXXX".
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
