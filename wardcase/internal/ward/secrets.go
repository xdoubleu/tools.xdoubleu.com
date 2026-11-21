package ward

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
)

// Secret names.
const (
	SecretAnthropic  = "anthropic"
	SecretOpenRouter = "openrouter"
	SecretOpenAI     = "openai"

	keychainService = "com.xdoubleu.wardcase"
	// notFound is `security`'s exit code for a missing item.
	notFound = 44
)

// ErrNoSecret means the secret was never stored.
var ErrNoSecret = errors.New("not stored")

// Secrets stores the model credentials.
type Secrets interface {
	Get(name string) (string, error)
	Set(name, value string) error
	// Delete removes a secret; one never stored is not an error.
	Delete(name string) error
}

// Keychain keeps secrets in the macOS login keychain through `security`, apart from
// factory-runner's.
type Keychain struct {
	// Run runs `security` with args; tests replace it.
	Run func(args ...string) (out []byte, code int, err error)
}

// NewKeychain returns the real keychain.
func NewKeychain() *Keychain {
	return &Keychain{Run: func(args ...string) ([]byte, int, error) {
		cmd := exec.CommandContext(context.Background(), "security", args...)
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return out.Bytes(), exit.ExitCode(), nil
		}
		return out.Bytes(), 0, err
	}}
}

// ValidSecret reports whether name is a secret wardcase uses.
func ValidSecret(name string) bool {
	return slices.Contains([]string{SecretAnthropic, SecretOpenRouter, SecretOpenAI}, name)
}

// Get implements Secrets.
func (k *Keychain) Get(name string) (string, error) {
	out, code, err := k.Run("find-generic-password", "-s", keychainService,
		"-a", name, "-w")
	switch {
	case err != nil:
		return "", fmt.Errorf("read keychain: %w", err)
	case code == notFound:
		return "", fmt.Errorf("the %s secret: %w (wardcase secret set %s)", name,
			ErrNoSecret, name)
	case code != 0:
		return "", fmt.Errorf("read keychain: security exited %d", code)
	}
	return strings.TrimSpace(string(out)), nil
}

// Set implements Secrets. `security` takes the value in argv, briefly visible to
// the user's own processes, as factory-runner's secrets are.
func (k *Keychain) Set(name, value string) error {
	_, code, err := k.Run("add-generic-password", "-U", "-s", keychainService, "-a", name,
		"-w", value)
	if err != nil {
		return fmt.Errorf("write keychain: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("write keychain: security exited %d", code)
	}
	return nil
}

// Delete implements Secrets.
func (k *Keychain) Delete(name string) error {
	_, code, err := k.Run("delete-generic-password", "-s", keychainService, "-a", name)
	if err != nil {
		return fmt.Errorf("write keychain: %w", err)
	}
	if code != 0 && code != notFound {
		return fmt.Errorf("write keychain: security exited %d", code)
	}
	return nil
}
