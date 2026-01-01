//nolint:nilnil,exhaustruct,revive,lll //ignore
package mocks

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/supabase-community/gotrue-go"
	"github.com/supabase-community/gotrue-go/types"
)

type MockedGoTrueClient struct {
	token string
}

func NewMockedGoTrueClient() gotrue.Client {
	return MockedGoTrueClient{}
}

func (client MockedGoTrueClient) WithCustomGoTrueURL(url string) gotrue.Client {
	return client
}

func (client MockedGoTrueClient) WithToken(token string) gotrue.Client {
	client.token = token
	return client
}

func (client MockedGoTrueClient) WithClient(httpClient http.Client) gotrue.Client {
	return client
}

func (client MockedGoTrueClient) AdminAudit(
	req types.AdminAuditRequest,
) (*types.AdminAuditResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) AdminGenerateLink(
	req types.AdminGenerateLinkRequest,
) (*types.AdminGenerateLinkResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) AdminListSSOProviders() (*types.AdminListSSOProvidersResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) AdminCreateSSOProvider(
	req types.AdminCreateSSOProviderRequest,
) (*types.AdminCreateSSOProviderResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) AdminGetSSOProvider(
	req types.AdminGetSSOProviderRequest,
) (*types.AdminGetSSOProviderResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) AdminUpdateSSOProvider(
	req types.AdminUpdateSSOProviderRequest,
) (*types.AdminUpdateSSOProviderResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) AdminDeleteSSOProvider(
	req types.AdminDeleteSSOProviderRequest,
) (*types.AdminDeleteSSOProviderResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) AdminCreateUser(
	req types.AdminCreateUserRequest,
) (*types.AdminCreateUserResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) AdminListUsers() (*types.AdminListUsersResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) AdminGetUser(
	req types.AdminGetUserRequest,
) (*types.AdminGetUserResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) AdminUpdateUser(
	req types.AdminUpdateUserRequest,
) (*types.AdminUpdateUserResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) AdminDeleteUser(
	req types.AdminDeleteUserRequest,
) error {
	return nil
}

func (client MockedGoTrueClient) AdminListUserFactors(
	req types.AdminListUserFactorsRequest,
) (*types.AdminListUserFactorsResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) AdminUpdateUserFactor(
	req types.AdminUpdateUserFactorRequest,
) (*types.AdminUpdateUserFactorResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) AdminDeleteUserFactor(
	req types.AdminDeleteUserFactorRequest,
) error {
	return nil
}

func (client MockedGoTrueClient) Authorize(
	req types.AuthorizeRequest,
) (*types.AuthorizeResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) EnrollFactor(
	req types.EnrollFactorRequest,
) (*types.EnrollFactorResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) ChallengeFactor(
	req types.ChallengeFactorRequest,
) (*types.ChallengeFactorResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) VerifyFactor(
	req types.VerifyFactorRequest,
) (*types.VerifyFactorResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) UnenrollFactor(
	req types.UnenrollFactorRequest,
) (*types.UnenrollFactorResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) HealthCheck() (*types.HealthCheckResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) Invite(
	req types.InviteRequest,
) (*types.InviteResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) Logout() error {
	return nil
}

func (client MockedGoTrueClient) Magiclink(req types.MagiclinkRequest) error {
	return nil
}

func (client MockedGoTrueClient) OTP(req types.OTPRequest) error {
	return nil
}

func (client MockedGoTrueClient) Reauthenticate() error {
	return nil
}

func (client MockedGoTrueClient) Recover(req types.RecoverRequest) error {
	return nil
}

func (client MockedGoTrueClient) GetSettings() (*types.SettingsResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) Signup(
	req types.SignupRequest,
) (*types.SignupResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) SignInWithEmailPassword(
	email, password string,
) (*types.TokenResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) SignInWithPhonePassword(
	phone, password string,
) (*types.TokenResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) RefreshToken(
	refreshToken string,
) (*types.TokenResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) Token(
	req types.TokenRequest,
) (*types.TokenResponse, error) {
	return &types.TokenResponse{
		Session: types.Session{
			AccessToken:  "access",
			RefreshToken: "refresh",
		},
	}, nil
}

func (client MockedGoTrueClient) GetUser() (*types.UserResponse, error) {
	if client.token == "access" {
		uuid, _ := uuid.Parse("4001e9cf-3fbe-4b09-863f-bd1654cfbf76")
		return &types.UserResponse{
			User: types.User{
				ID:    uuid,
				Email: "user@example.com",
			},
		}, nil
	}

	return nil, nil
}

func (client MockedGoTrueClient) UpdateUser(
	req types.UpdateUserRequest,
) (*types.UpdateUserResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) Verify(
	req types.VerifyRequest,
) (*types.VerifyResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) VerifyForUser(
	req types.VerifyForUserRequest,
) (*types.VerifyForUserResponse, error) {
	return nil, nil
}

func (client MockedGoTrueClient) SAMLMetadata() ([]byte, error) {
	return nil, nil
}

func (client MockedGoTrueClient) SAMLACS(req *http.Request) (*http.Response, error) {
	return nil, nil
}

func (client MockedGoTrueClient) SSO(req types.SSORequest) (*types.SSOResponse, error) {
	return nil, nil
}
