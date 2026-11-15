//nolint:exhaustruct // test doubles set only what each case needs
package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/crypto"
)

type fakeImportStore struct {
	candidates []ImportCandidate
	listErr    error
	setErr     error
	subjects   map[string]string
}

func (s *fakeImportStore) ListImportCandidates(
	context.Context,
) ([]ImportCandidate, error) {
	return s.candidates, s.listErr
}

func (s *fakeImportStore) SetOIDCSubject(_ context.Context, id, subject string) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.subjects[id] = subject
	return nil
}

type importCall struct{ email, name, hash, totp string }

type fakeImporter struct {
	calls   []importCall
	failFor string
}

func (i *fakeImporter) ImportHuman(
	_ context.Context,
	email, name, hash, totp string,
) (string, error) {
	if email == i.failFor {
		return "", errProvider
	}
	i.calls = append(i.calls, importCall{email, name, hash, totp})
	return "zid-" + email, nil
}

func newImportSealer(t *testing.T) *crypto.Sealer {
	t.Helper()
	sealer, err := crypto.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	require.NoError(t, err)
	return sealer
}

func sealed(t *testing.T, s *crypto.Sealer, secret string) string {
	t.Helper()
	out, err := s.Encrypt([]byte(secret))
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(out)
}

func TestImportJob_ImportsHashAndUnsealedTOTP(t *testing.T) {
	sealer := newImportSealer(t)
	store := &fakeImportStore{
		subjects: map[string]string{},
		candidates: []ImportCandidate{
			{UserID: "u1", Email: "a@x.com", DisplayName: "A", PasswordHash: "h1",
				SealedTOTP: sealed(t, sealer, "SECRET")},
			{UserID: "u2", Email: "b@x.com", PasswordHash: "h2"},
		},
	}
	importer := &fakeImporter{}
	job := NewImportJob(store, importer, sealer)

	require.NoError(t, job.Run(context.Background(), slog.Default()))

	assert.Equal(t, []importCall{
		{"a@x.com", "A", "h1", "SECRET"}, {"b@x.com", "", "h2", ""},
	}, importer.calls)
	assert.Equal(t, map[string]string{
		"u1": "zid-a@x.com", "u2": "zid-b@x.com",
	}, store.subjects)
}

func TestImportJob_FailureDoesNotBlockOthers(t *testing.T) {
	store := &fakeImportStore{
		subjects: map[string]string{},
		candidates: []ImportCandidate{
			{UserID: "u1", Email: "bad@x.com", PasswordHash: "h"},
			{UserID: "u2", Email: "ok@x.com", PasswordHash: "h"},
		},
	}
	job := NewImportJob(store, &fakeImporter{failFor: "bad@x.com"}, newImportSealer(t))

	err := job.Run(context.Background(), slog.Default())

	require.ErrorIs(t, err, errProvider)
	assert.Equal(t, map[string]string{"u2": "zid-ok@x.com"}, store.subjects)
}

func TestImportJob_BadTOTPSecretFailsThatUser(t *testing.T) {
	sealer := newImportSealer(t)
	for _, sealedSecret := range []string{"!!notbase64", "AAAA"} {
		store := &fakeImportStore{
			subjects: map[string]string{},
			candidates: []ImportCandidate{
				{UserID: "u1", Email: "a@x.com", PasswordHash: "h",
					SealedTOTP: sealedSecret},
			},
		}
		importer := &fakeImporter{}

		err := NewImportJob(store, importer, sealer).Run(
			context.Background(), slog.Default(),
		)

		require.Error(t, err)
		assert.Empty(t, importer.calls)
		assert.Empty(t, store.subjects)
	}
}

func TestImportJob_StoreErrors(t *testing.T) {
	boom := errors.New("db down")
	job := NewImportJob(
		&fakeImportStore{listErr: boom}, &fakeImporter{}, newImportSealer(t),
	)
	require.ErrorIs(t, job.Run(context.Background(), slog.Default()), boom)

	store := &fakeImportStore{
		setErr:     boom,
		candidates: []ImportCandidate{{UserID: "u1", Email: "a@x.com"}},
	}
	job = NewImportJob(store, &fakeImporter{}, newImportSealer(t))
	require.ErrorIs(t, job.Run(context.Background(), slog.Default()), boom)
}

func TestImportJob_Identity(t *testing.T) {
	job := NewImportJob(nil, nil, nil)
	assert.NotEmpty(t, job.ID())
	assert.Equal(t, time.Hour, job.RunEvery())
}
