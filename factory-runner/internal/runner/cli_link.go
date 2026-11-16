package runner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// CLIDirs lists, in order, where the `factory-runner` command is linked on PATH:
// the first one that is writable and not holding someone else's file wins. Tests
// point it somewhere temporary.
//
//nolint:gochecknoglobals // swapped by tests
var CLIDirs = func(homeDir string) []string {
	return []string{
		"/usr/local/bin", "/opt/homebrew/bin", filepath.Join(homeDir, ".local", "bin"),
	}
}

// LinkCLI makes `factory-runner` runnable from any shell, headless, by linking it
// to binary inside the installed app, and returns the link. The link is the same
// binary the menu bar runs, so both write one config. An existing link is
// repointed; a regular file of that name is left alone.
func LinkCLI(homeDir, binary string) (string, error) {
	var errs []error
	for _, dir := range CLIDirs(homeDir) {
		link := filepath.Join(dir, "factory-runner")
		if err := linkInto(dir, link, binary); err != nil {
			errs = append(errs, err)
			continue
		}
		return link, nil
	}
	return "", fmt.Errorf("link the factory-runner command: %w", errors.Join(errs...))
}

func linkInto(dir, link, binary string) error {
	if info, err := os.Lstat(link); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("%s exists and is not a link", link)
		}
		if cur, readErr := os.Readlink(link); readErr == nil && cur == binary {
			return nil
		}
	}
	if !writable(dir) {
		return fmt.Errorf("%s is not writable", dir)
	}
	tmp := link + ".new"
	_ = os.Remove(tmp)
	if err := os.Symlink(binary, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, link); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
