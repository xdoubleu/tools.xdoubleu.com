package auth

import (
	"bufio"
	"context"
	"crypto/sha1" //nolint:gosec //HIBP's range API is keyed by SHA-1
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	// maxPasswordBytes is bcrypt's input limit; it ignores anything beyond.
	maxPasswordBytes = 72
	pwnedTimeout     = 2 * time.Second
	pwnedPrefixLen   = 5
	maxRangeBody     = 4 << 20
	// defaultRangeEndpoint is Have I Been Pwned's k-anonymity range API.
	defaultRangeEndpoint = "https://api.pwnedpasswords.com/range/"
)

// ErrPasswordTooLong rejects a new password over maxPasswordBytes.
var ErrPasswordTooLong = fmt.Errorf(
	"password must be at most %d bytes", maxPasswordBytes,
)

// ErrPasswordBreached rejects a new password found in a known data breach.
var ErrPasswordBreached = errors.New(
	"this password appeared in a known data breach, choose another",
)

// IsPasswordPolicyError reports whether err is a rejected new password.
func IsPasswordPolicyError(err error) bool {
	return errors.Is(err, ErrPasswordTooShort) ||
		errors.Is(err, ErrPasswordTooLong) ||
		errors.Is(err, ErrPasswordBreached)
}

// checkPasswordLength applies the length rules, which need no I/O.
func checkPasswordLength(password string) error {
	if len(password) < minPasswordLength {
		return ErrPasswordTooShort
	}
	if len(password) > maxPasswordBytes {
		return ErrPasswordTooLong
	}
	return nil
}

// checkPasswordBreached asks the range API whether password is known breached.
// Only the first five hex characters of its SHA-1 leave the server. An
// unreachable or slow API allows the password.
func (service *LocalService) checkPasswordBreached(
	ctx context.Context,
	password string,
) error {
	if service.PwnedRangeURL == "" {
		return nil
	}

	sum := sha1.Sum([]byte(password)) //nolint:gosec //see import
	digest := strings.ToUpper(hex.EncodeToString(sum[:]))
	prefix, suffix := digest[:pwnedPrefixLen], digest[pwnedPrefixLen:]

	breached, err := service.pwnedHasSuffix(ctx, prefix, suffix)
	if err != nil {
		slog.Default().WarnContext(ctx, "breached-password check skipped", "error", err)
		return nil
	}
	if breached {
		return ErrPasswordBreached
	}
	return nil
}

func (service *LocalService) pwnedHasSuffix(
	ctx context.Context,
	prefix, suffix string,
) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, pwnedTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, service.PwnedRangeURL+prefix, nil,
	)
	if err != nil {
		return false, err
	}
	req.Header.Set("Add-Padding", "true")

	resp, err := (&http.Client{Timeout: pwnedTimeout}).Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("pwned range API status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(io.LimitReader(resp.Body, maxRangeBody))
	for scanner.Scan() {
		hash, count, _ := strings.Cut(strings.TrimSpace(scanner.Text()), ":")
		// Padding entries have a count of 0.
		if strings.EqualFold(hash, suffix) && count != "0" {
			return true, nil
		}
	}
	return false, scanner.Err()
}
