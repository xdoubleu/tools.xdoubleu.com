package runner

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

const (
	keychainService = "com.xdoubleu.tools.factory-runner"
	keychainAccount = "device-token"
	// securityItemNotFound is `security`'s exit status for a missing item.
	securityItemNotFound = 44
)

// ErrNoToken means the runner isn't paired on this Mac.
var ErrNoToken = errors.New("not paired: run `factory-runner pair <code>`")

// TokenStore keeps the device token.
type TokenStore interface {
	Get() (string, error)
	Set(token string) error
}

// Keychain stores the token in the macOS login keychain through `security`.
// The token is briefly in the argument list of that process, readable by this
// user only.
type Keychain struct {
	run func(args ...string) ([]byte, int, error)
}

// NewKeychain returns the real keychain.
func NewKeychain() *Keychain {
	return &Keychain{run: runSecurity}
}

func runSecurity(args ...string) ([]byte, int, error) {
	out, err := exec.CommandContext(context.Background(), "security", args...).Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return out, exit.ExitCode(), err
	}
	return out, 0, err
}

// Get returns the stored token.
func (k *Keychain) Get() (string, error) {
	out, code, err := k.run("find-generic-password",
		"-s", keychainService, "-a", keychainAccount, "-w")
	if code == securityItemNotFound {
		return "", ErrNoToken
	}
	if err != nil {
		return "", fmt.Errorf("read keychain: %w", err)
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", ErrNoToken
	}
	return token, nil
}

// Set stores the token, replacing any earlier one.
func (k *Keychain) Set(token string) error {
	_, _, err := k.run("add-generic-password", "-U",
		"-s", keychainService, "-a", keychainAccount, "-w", token)
	if err != nil {
		return fmt.Errorf("write keychain: %w", err)
	}
	return nil
}
