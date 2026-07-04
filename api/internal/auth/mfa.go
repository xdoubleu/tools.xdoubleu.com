package auth

import (
	"errors"

	"github.com/google/uuid"
	"github.com/supabase-community/auth-go/types"
	"github.com/xdoubleu/essentia/v4/pkg/errortools"
)

func (service *GoTrueService) UnenrollTOTP(
	accessToken string,
	factorID uuid.UUID,
) error {
	_, err := service.client.WithToken(accessToken).UnenrollFactor(
		types.UnenrollFactorRequest{FactorID: factorID},
	)
	return err
}

// HasVerifiedTOTP returns the factor ID of the first verified TOTP factor, or
// (zero, false) when the user has not enrolled MFA yet.
func (service *GoTrueService) HasVerifiedTOTP(
	accessToken string,
) (uuid.UUID, bool) {
	resp, err := service.client.WithToken(accessToken).GetUser()
	if err != nil {
		return uuid.UUID{}, false
	}
	for _, f := range resp.Factors {
		if f.FactorType == "totp" && f.Status == "verified" {
			return f.ID, true
		}
	}
	return uuid.UUID{}, false
}

// EnrollTOTP begins TOTP enrollment for the given access token and returns the
// QR code SVG, fallback secret, and factor ID.
// Any pre-existing unverified TOTP factor is unenrolled first to avoid the
// friendly-name conflict error Supabase returns on repeated enrollment attempts.
func (service *GoTrueService) EnrollTOTP(
	accessToken string,
) (*types.EnrollFactorResponse, error) {
	authedClient := service.client.WithToken(accessToken)

	// Clean up any leftover unverified factor from a previous partial enrollment.
	if resp, err := authedClient.GetUser(); err == nil {
		for _, f := range resp.Factors {
			if f.FactorType == "totp" && f.Status == "unverified" {
				_, _ = authedClient.UnenrollFactor(
					types.UnenrollFactorRequest{FactorID: f.ID},
				)
			}
		}
	}

	return authedClient.EnrollFactor(
		//nolint:exhaustruct //issuer and friendlyName are optional
		types.EnrollFactorRequest{
			FactorType: types.FactorTypeTOTP,
		},
	)
}

// ChallengeMFA creates a challenge for the given factor and returns its ID.
func (service *GoTrueService) ChallengeMFA(
	accessToken string,
	factorID uuid.UUID,
) (*types.ChallengeFactorResponse, error) {
	return service.client.WithToken(accessToken).ChallengeFactor(

		types.ChallengeFactorRequest{FactorID: factorID},
	)
}

// VerifyMFA completes the MFA challenge and returns new aal2 access and refresh tokens.
func (service *GoTrueService) VerifyMFA(
	accessToken string,
	factorID uuid.UUID,
	challengeID uuid.UUID,
	code string,
) (*string, *string, error) {
	resp, err := service.client.WithToken(accessToken).VerifyFactor(
		types.VerifyFactorRequest{
			FactorID:    factorID,
			ChallengeID: challengeID,
			Code:        code,
		},
	)
	if err != nil {
		return nil, nil, errortools.NewUnauthorizedError(errors.New("invalid MFA code"))
	}
	return &resp.AccessToken, &resp.RefreshToken, nil
}
