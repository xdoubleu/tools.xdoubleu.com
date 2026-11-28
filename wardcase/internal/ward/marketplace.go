package ward

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"tools.xdoubleu.com/wardcase/vm"
)

const (
	marketplacesDir = "marketplaces"
	// marketplaceTTL is how long a floating ref's cache is reused before the host
	// fetches again.
	marketplaceTTL = time.Hour
	// stampFile, inside .git so the mount doesn't show it, records what was fetched.
	stampFile    = "wardcase-fetched"
	manifestPath = ".claude-plugin/marketplace.json"
)

var (
	marketplaceName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	pluginName      = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	refPattern      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/@+-]*$`)
	commitPattern   = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
	safePath        = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
)

// MarketplaceConfig is one skills marketplace of the device: a Git repository
// holding a `.claude-plugin/marketplace.json`, fetched on the host.
type MarketplaceConfig struct {
	// Name is the marketplace's name in the harness and in the repo file's
	// "marketplace/plugin" entries.
	Name string `toml:"name"`
	// URL is the Git URL, fetched with the host's own credentials.
	URL string `toml:"url"`
	// Ref is a branch, tag or commit; empty is the default branch. A full commit
	// is pinned: it is fetched once and never refreshed.
	Ref string `toml:"ref"`
	// Plugins are the plugins the sessions get.
	Plugins []string `toml:"plugins"`
}

// MarketplaceUse is a marketplace a run got, at the commit it had.
type MarketplaceUse struct {
	Name, Commit string
}

func (c Config) validateMarketplaces() error {
	seen := map[string]bool{}
	for _, m := range c.Marketplace {
		if !marketplaceName.MatchString(m.Name) {
			return fmt.Errorf("marketplace: %q is not a name (lower-case letters, digits "+
				"and dashes)", m.Name)
		}
		if seen[m.Name] {
			return fmt.Errorf("marketplace: %q is listed twice", m.Name)
		}
		seen[m.Name] = true
		if err := validMarketplaceURL(m.URL); err != nil {
			return fmt.Errorf("marketplace %s: %w", m.Name, err)
		}
		if m.Ref != "" && !refPattern.MatchString(m.Ref) {
			return fmt.Errorf("marketplace %s: ref %q is not a branch, tag or commit",
				m.Name, m.Ref)
		}
		for _, p := range m.Plugins {
			if !pluginName.MatchString(p) {
				return fmt.Errorf("marketplace %s: %q is not a plugin name", m.Name, p)
			}
		}
	}
	return nil
}

// validMarketplaceURL refuses what git would read as an option, a credential in
// the URL (the cache is mounted into the VM), and whitespace.
func validMarketplaceURL(raw string) error {
	if raw == "" || strings.HasPrefix(raw, "-") || strings.ContainsAny(raw, " \t\r\n") {
		return fmt.Errorf("url %q is not a Git URL", raw)
	}
	if u, err := url.Parse(raw); err == nil && u.User != nil {
		if _, hasPassword := u.User.Password(); hasPassword {
			return errors.New("url holds a password; use the host's Git credentials")
		}
	}
	return nil
}

// skillLog collects the marketplaces a session's runs used.
type skillLog struct {
	mu   sync.Mutex
	used []MarketplaceUse
}

// addSkills fetches the device's marketplaces into the cache and returns their
// read-only mounts and the VM environment that enables the plugins, narrowed by the
// repo file. A marketplace that can't be fetched or read is skipped with a note, so
// a run goes on without those skills.
func (r *Runner) addSkills(
	ctx context.Context, env map[string]string, checkout string,
) ([]vm.Mount, error) {
	if len(r.Config.Marketplace) == 0 {
		return nil, nil
	}
	repo, err := LoadRepoConfig(checkout)
	if err != nil {
		return nil, err
	}
	enabled := r.enabledPlugins(repo)
	var mounts []vm.Mount
	var used []MarketplaceUse
	var names, plugins, paths []string
	for _, m := range r.Config.Marketplace {
		if len(enabled[m.Name]) == 0 {
			continue
		}
		dir, commit, fetchErr := r.fetchMarketplace(ctx, m)
		if fetchErr != nil {
			fmt.Fprintf(r.Notes, "wardcase: marketplace %s is skipped: %v\n", m.Name, fetchErr)
			continue
		}
		dirs, listed := r.pluginDirs(m, dir, enabled[m.Name])
		if len(listed) == 0 {
			continue
		}
		target := path.Join(vm.MarketplacesRoot, m.Name)
		mounts = append(mounts, vm.Mount{Source: dir, Target: target, ReadOnly: true})
		names = append(names, m.Name)
		for _, p := range listed {
			plugins = append(plugins, p+"@"+m.Name)
		}
		for _, d := range dirs {
			paths = append(paths, path.Join(target, d, "skills"))
		}
		used = append(used, MarketplaceUse{Name: m.Name, Commit: commit})
		fmt.Fprintf(r.Notes, "wardcase: marketplace %s at %s\n", m.Name, commit)
	}
	if len(names) > 0 {
		env[vm.MarketplacesEnv] = strings.Join(names, " ")
		env[vm.PluginsEnv] = strings.Join(plugins, " ")
		if len(paths) > 0 {
			env[vm.SkillPathsEnv] = strings.Join(paths, " ")
		}
	}
	r.recordSkills(used)
	return mounts, nil
}

// enabledPlugins is each marketplace's plugins the run gets: the device's list,
// narrowed to the repo file's "marketplace/plugin" entries when it has plugins.
func (r *Runner) enabledPlugins(repo RepoConfig) map[string][]string {
	out := map[string][]string{}
	for _, m := range r.Config.Marketplace {
		out[m.Name] = m.Plugins
	}
	if repo.Plugins == nil {
		return out
	}
	narrowed := map[string][]string{}
	for _, entry := range *repo.Plugins {
		mk, plugin, _ := strings.Cut(entry, "/")
		if !slices.Contains(out[mk], plugin) {
			fmt.Fprintf(r.Notes, "wardcase: the repo lists plugin %s, which the device "+
				"does not configure\n", entry)
			continue
		}
		narrowed[mk] = append(narrowed[mk], plugin)
	}
	return narrowed
}

// recordSkills keeps what a run used for the caller's result.
func (r *Runner) recordSkills(used []MarketplaceUse) {
	if r.skills == nil {
		return
	}
	r.skills.mu.Lock()
	defer r.skills.mu.Unlock()
	for _, u := range used {
		if !slices.Contains(r.skills.used, u) {
			r.skills.used = append(r.skills.used, u)
		}
	}
}

// pluginDirs resolves the wanted plugins against the marketplace's manifest: the
// plugin directories relative to the marketplace, and the plugin names found.
func (r *Runner) pluginDirs(
	m MarketplaceConfig, dir string, want []string,
) ([]string, []string) {
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(manifestPath)))
	if err != nil {
		fmt.Fprintf(r.Notes, "wardcase: marketplace %s has no %s\n", m.Name, manifestPath)
		return nil, nil
	}
	var manifest struct {
		Plugins []struct {
			Name   string          `json:"name"`
			Source json.RawMessage `json:"source"`
		} `json:"plugins"`
	}
	if json.Unmarshal(data, &manifest) != nil {
		fmt.Fprintf(r.Notes, "wardcase: marketplace %s has an unreadable %s\n",
			m.Name, manifestPath)
		return nil, nil
	}
	var dirs, listed []string
	for _, name := range want {
		i := slices.IndexFunc(manifest.Plugins, func(p struct {
			Name   string          `json:"name"`
			Source json.RawMessage `json:"source"`
		}) bool {
			return p.Name == name
		})
		if i < 0 {
			fmt.Fprintf(r.Notes, "wardcase: marketplace %s has no plugin %s\n", m.Name, name)
			continue
		}
		listed = append(listed, name)
		var source string
		if json.Unmarshal(manifest.Plugins[i].Source, &source) != nil {
			continue // not a path inside the marketplace: Claude resolves it, OpenCode skips it
		}
		rel := path.Clean(source)
		if rel == "." || strings.HasPrefix(rel, "..") || path.IsAbs(rel) ||
			!safePath.MatchString(rel) {
			continue
		}
		if info, statErr := os.Stat(filepath.Join(dir, filepath.FromSlash(rel), "skills")); statErr == nil &&
			info.IsDir() {
			dirs = append(dirs, rel)
		}
	}
	return dirs, listed
}

// fetchMarketplace brings m's cache up to date and returns its directory and the
// commit checked out. A floating ref is fetched at most once an hour; a pinned
// commit once. When the fetch fails, a cache of the same source is reused with a
// note, so an offline host keeps its skills.
func (r *Runner) fetchMarketplace(
	ctx context.Context, m MarketplaceConfig,
) (string, string, error) {
	dir := filepath.Join(r.StateDir, marketplacesDir, m.Name)
	if err := os.MkdirAll(dir, stateDirPerm); err != nil {
		return "", "", fmt.Errorf("cache directory: %w", err)
	}
	git := func(args ...string) (string, error) {
		return r.Git(ctx, dir, append(safeGit(), args...)...)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		if _, initErr := git("init", "--quiet"); initErr != nil {
			return "", "", initErr
		}
	}
	stamp := filepath.Join(dir, ".git", stampFile)
	source := m.URL + "\n" + m.Ref
	head, headErr := git("rev-parse", "--verify", "--quiet", "HEAD")
	head = strings.TrimSpace(head)
	cached := headErr == nil && head != ""
	same := false
	if data, err := os.ReadFile(stamp); err == nil {
		same = string(data) == source
	}
	if cached && same {
		info, statErr := os.Stat(stamp)
		fresh := statErr == nil && r.now().Sub(info.ModTime()) < marketplaceTTL
		if commitPattern.MatchString(m.Ref) || fresh {
			return dir, head, nil
		}
	}
	commit, err := r.pullMarketplace(git, m)
	if err != nil {
		if cached && same {
			fmt.Fprintf(r.Notes, "wardcase: marketplace %s could not be fetched (%v); "+
				"using the cache at %s\n", m.Name, err, head)
			return dir, head, nil
		}
		return "", "", err
	}
	if err = os.WriteFile(stamp, []byte(source), filePerm); err != nil {
		return "", "", fmt.Errorf("record the fetch: %w", err)
	}
	return dir, commit, nil
}

func (r *Runner) pullMarketplace(
	git func(args ...string) (string, error), m MarketplaceConfig,
) (string, error) {
	ref := m.Ref
	if ref == "" {
		ref = "HEAD"
	}
	if _, err := git("fetch", "--quiet", "--depth", "1", "--no-tags", "--", m.URL, ref); err != nil {
		return "", err
	}
	if _, err := git("checkout", "--quiet", "--force", "--detach", "FETCH_HEAD"); err != nil {
		return "", err
	}
	if _, err := git("clean", "--quiet", "-dfx"); err != nil {
		return "", err
	}
	head, err := git("rev-parse", "HEAD")
	return strings.TrimSpace(head), err
}
