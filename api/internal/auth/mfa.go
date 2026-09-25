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

	sealed, err := base64.StdEncoding.DecodeString(factor.Secret)
	if err != nil {
		return nil, nil, err
	}
	secret, err := service.sealer.Decrypt(sealed)
	if err != nil {
		return nil, nil, err
	}

	// A malformed code (e.g. a recovery code) errors here; ignore it so the
	// recovery-code fallback still runs.
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
			_ = service.usersStore.MarkRecoveryCodeUsed(ctx, rc.ID)
			return true
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

	c, err := service.parseAccessToken(accessToken)
	if err != nil {
		return err
	}

	if err = service.usersStore.DeleteAllTOTPFactors(ctx, c.Subject); err != nil {
		return err
	}
	return service.usersStore.DeleteRecoveryCodes(ctx, c.Subject)
}

// GenerateRecoveryCodes replaces the user's recovery codes with 10 new ones,
// returning the plaintext once.
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
