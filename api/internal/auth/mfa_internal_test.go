package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// factorLookupStore fails or finds the verified-factor lookup; every other
// usersStore method is unused here.
type factorLookupStore struct {
	usersStore
	err error
}

func (s factorLookupStore) GetVerifiedTOTPFactor(
	context.Context, string,
) (*TOTPFactor, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &TOTPFactor{}, nil //nolint:exhaustruct // presence is all that matters
}

func TestRequireMFAIfEnrolled(t *testing.T) {
	dbErr := errors.New("db down")
	cases := []struct {
		name  string
		store factorLookupStore
		aal   string
		want  error
	}{
		{"aal2 skips the lookup", factorLookupStore{nil, dbErr}, aal2, nil},
		{"no factor", factorLookupStore{nil, errNotFound}, aal1, nil},
		{"enrolled", factorLookupStore{nil, nil}, aal1, errMFARequired},
		{"lookup failure fails closed", factorLookupStore{nil, dbErr}, aal1, dbErr},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			//nolint:exhaustruct // only usersStore is read
			service := &LocalService{usersStore: tc.store}
			err := service.requireMFAIfEnrolled(t.Context(), "user", tc.aal)
			if tc.want == nil {
				require.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestParseSessionToken_LookupFailureFailsClosed(t *testing.T) {
	//nolint:exhaustruct // only the token and factor-lookup fields are read
	service := &LocalService{
		usersStore:   factorLookupStore{nil, errors.New("db down")},
		jwtSecret:    []byte("secret"),
		accessExpiry: "1h",
	}
	token, err := service.mintAccessToken("user", aal2)
	require.NoError(t, err)

	_, _, err = service.parseSessionToken(t.Context(), token)
	require.Error(t, err)
}
