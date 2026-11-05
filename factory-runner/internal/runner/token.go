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

// ErrNoSecret means a credential the runner injects has not been stored.
var ErrNoSecret = errors.New("secret not stored")

// Secret names the runner stores besides the device token.
const (
	SecretClaude     = "claude"
	SecretOpenRouter = "openrouter"
)

// SecretStore keeps the credentials jobs use: the Claude setup-token and the
// OpenRouter key.
type SecretStore interface {
	GetSecret(name string) (string, error)
	SetSecret(name, value string) error
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

// GetSecret returns a stored credential.
func (k *Keychain) GetSecret(name string) (string, error) {
	if !validSecretName(name) {
		return "", fmt.Errorf("unknown secret %q", name)
	}
	out, code, err := k.run("find-generic-password",
		"-s", keychainService, "-a", "secret-"+name, "-w")
	if code == securityItemNotFound {
		return "", fmt.Errorf("%w: %s (run `factory-runner config secret %s`)",
			ErrNoSecret, name, name)
	}
	if err != nil {
		return "", fmt.Errorf("read keychain: %w", err)
	}
	value := strings.TrimSpace(string(out))
	if value == "" {
		return "", fmt.Errorf("%w: %s", ErrNoSecret, name)
	}
	return value, nil
}

// SetSecret stores a credential, replacing any earlier one. Like the device token it
// is briefly in the argument list of `security`, readable by this user only.
func (k *Keychain) SetSecret(name, value string) error {
	if !validSecretName(name) {
		return fmt.Errorf("unknown secret %q", name)
	}
	if value == "" || strings.ContainsAny(value, "\n\r") {
		return errors.New("a secret is one non-empty line")
	}
	if _, _, err := k.run("add-generic-password", "-U",
		"-s", keychainService, "-a", "secret-"+name, "-w", value); err != nil {
		return fmt.Errorf("write keychain: %w", err)
	}
	return nil
}

func validSecretName(name string) bool {
	return name == SecretClaude || name == SecretOpenRouter
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
