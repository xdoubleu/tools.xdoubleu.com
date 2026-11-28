package ward_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/internal/ward"
	"tools.xdoubleu.com/wardcase/vm"
)

const devSkills = vm.MarketplacesRoot + "/skills/plugins/dev/skills"

const manifest = `{"name":"skills","plugins":[
 {"name":"dev","source":"./plugins/dev"},
 {"name":"other","source":"./plugins/other"},
 {"name":"remote","source":{"source":"github","repo":"a/b"}}]}`

// marketplaceRepo is a bare Git repository with a marketplace of two plugins, and
// the working clone that pushes new commits to it.
type marketplaceRepo struct {
	t     *testing.T
	bare  string
	work  string
	count int
}

func newMarketplaceRepo(t *testing.T) *marketplaceRepo {
	t.Helper()
	m := &marketplaceRepo{t: t, bare: t.TempDir(), work: t.TempDir(), count: 0}
	gitIn(t, m.bare, "init", "-q", "--bare", "-b", "main")
	gitIn(t, m.work, "init", "-q", "-b", "main")
	for _, p := range []string{"dev", "other"} {
		dir := filepath.Join(m.work, "plugins", p, "skills", "s")
		require.NoError(t, os.MkdirAll(dir, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(p), 0o600))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(m.work, ".claude-plugin"), 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(m.work, ".claude-plugin", "marketplace.json"), []byte(manifest), 0o600))
	m.commit()
	return m
}

// commit adds a commit and pushes it.
func (m *marketplaceRepo) commit() {
	m.t.Helper()
	m.count++
	write(m.t, m.work, "change"+strings.Repeat("x", m.count))
	gitIn(m.t, m.work, "add", ".")
	gitIn(m.t, m.work, "commit", "-q", "-m", "change")
	gitIn(m.t, m.work, "push", "-q", m.bare, "HEAD:refs/heads/main")
}

// runWith runs one session with the given marketplaces on a clock that is offset
// from now, and returns the run's VM spec and result.
func runWith(
	t *testing.T, sf *sessionFixture, offset time.Duration,
	marketplaces ...ward.MarketplaceConfig,
) (vm.SandboxSpec, ward.SessionResult) {
	t.Helper()
	sf.f.r.Now = func() time.Time { return time.Now().Add(offset) }
	sf.f.box.act = func(int, run, io.Writer) int { return 0 }
	req := sf.request("agent/" + strings.ReplaceAll(t.Name(), "/", "-") + offset.String())
	req.Marketplaces = marketplaces
	res, err := sf.f.r.RunSession(context.Background(), req)
	require.NoError(t, err)
	return sf.f.box.runs[len(sf.f.box.runs)-1].spec, res
}

func skillsMarketplace(url, ref string, plugins ...string) ward.MarketplaceConfig {
	return ward.MarketplaceConfig{Name: "skills", URL: url, Ref: ref, Plugins: plugins}
}

func TestAMarketplaceIsFetchedOnTheHostAndMountedReadOnly(t *testing.T) {
	repo := newMarketplaceRepo(t)
	sf := newSessionFixture(t)

	spec, res := runWith(t, sf, 0, skillsMarketplace(repo.bare, "", "dev"))

	var mount *vm.Mount
	for i := range spec.Mounts {
		if spec.Mounts[i].Target == vm.MarketplacesRoot+"/skills" {
			mount = &spec.Mounts[i]
		}
	}
	require.NotNil(t, mount)
	assert.True(t, mount.ReadOnly)
	skill := filepath.Join(mount.Source, "plugins", "dev", "skills", "s", "SKILL.md")
	assert.FileExists(t, skill)
	assert.Equal(t, "skills", spec.Env[vm.MarketplacesEnv])
	assert.Equal(t, "dev@skills", spec.Env[vm.PluginsEnv])
	assert.Equal(t, devSkills, spec.Env[vm.SkillPathsEnv])
	head := gitIn(t, repo.work, "rev-parse", "HEAD")
	used := []ward.MarketplaceUse{{Name: "skills", Commit: head}}
	assert.Equal(t, used, res.Marketplaces)
	assert.Contains(t, sf.f.notes.String(), "marketplace skills at "+head)
}

func TestNoMarketplaceGivesNoSkills(t *testing.T) {
	sf := newSessionFixture(t)
	spec, res := runWith(t, sf, 0)
	assert.NotContains(t, spec.Env, vm.MarketplacesEnv)
	assert.NotContains(t, spec.Env, vm.PluginsEnv)
	assert.NotContains(t, spec.Env, vm.SkillPathsEnv)
	assert.Empty(t, res.Marketplaces)
	for _, m := range spec.Mounts {
		assert.NotContains(t, m.Target, vm.MarketplacesRoot)
	}
}

func TestAFloatingRefIsFetchedAgainOnlyAfterAnHour(t *testing.T) {
	repo := newMarketplaceRepo(t)
	sf := newSessionFixture(t)
	mk := skillsMarketplace(repo.bare, "", "dev")
	_, first := runWith(t, sf, 0, mk)

	repo.commit()
	_, soon := runWith(t, sf, 10*time.Minute, mk)
	assert.Equal(t, first.Marketplaces, soon.Marketplaces, "the cache is fresh")

	_, later := runWith(t, sf, 2*time.Hour, mk)
	assert.Equal(t, gitIn(t, repo.work, "rev-parse", "HEAD"), later.Marketplaces[0].Commit)
}

func TestAPinnedCommitIsFetchedOnceAndNeverRefreshed(t *testing.T) {
	repo := newMarketplaceRepo(t)
	pinned := gitIn(t, repo.work, "rev-parse", "HEAD")
	repo.commit()
	sf := newSessionFixture(t)
	mk := skillsMarketplace(repo.bare, pinned, "dev")

	_, first := runWith(t, sf, 0, mk)
	require.Equal(t, pinned, first.Marketplaces[0].Commit)

	require.NoError(t, os.RemoveAll(repo.bare), "the source is gone")
	_, later := runWith(t, sf, 5*time.Hour, mk)
	assert.Equal(t, pinned, later.Marketplaces[0].Commit)
	assert.NotContains(t, sf.f.notes.String(), "could not be fetched")
}

func TestAFloatingRefTracksABranch(t *testing.T) {
	repo := newMarketplaceRepo(t)
	gitIn(t, repo.work, "push", "-q", repo.bare, "HEAD:refs/heads/next")
	next := gitIn(t, repo.work, "rev-parse", "HEAD")
	repo.commit()
	sf := newSessionFixture(t)

	_, res := runWith(t, sf, 0, skillsMarketplace(repo.bare, "next", "dev"))
	assert.Equal(t, next, res.Marketplaces[0].Commit)
}

func TestAnOfflineHostReusesTheCacheWithAWarning(t *testing.T) {
	repo := newMarketplaceRepo(t)
	sf := newSessionFixture(t)
	mk := skillsMarketplace(repo.bare, "", "dev")
	_, first := runWith(t, sf, 0, mk)

	require.NoError(t, os.RemoveAll(repo.bare))
	spec, again := runWith(t, sf, 2*time.Hour, mk)

	assert.Equal(t, first.Marketplaces, again.Marketplaces)
	assert.Equal(t, "dev@skills", spec.Env[vm.PluginsEnv])
	assert.Contains(t, sf.f.notes.String(), "could not be fetched")
	assert.Contains(t, sf.f.notes.String(), "using the cache")
}

func TestAMarketplaceThatCannotBeFetchedAndHasNoCacheIsSkipped(t *testing.T) {
	sf := newSessionFixture(t)
	gone := filepath.Join(t.TempDir(), "nope")
	spec, res := runWith(t, sf, 0, skillsMarketplace(gone, "", "dev"))

	assert.NotContains(t, spec.Env, vm.MarketplacesEnv)
	assert.Empty(t, res.Marketplaces)
	assert.Contains(t, sf.f.notes.String(), "marketplace skills is skipped")
}

func TestAChangedSourceIsNotServedFromTheOldCache(t *testing.T) {
	repo := newMarketplaceRepo(t)
	sf := newSessionFixture(t)
	_, _ = runWith(t, sf, 0, skillsMarketplace(repo.bare, "", "dev"))

	other := newMarketplaceRepo(t)
	require.NoError(t, os.RemoveAll(repo.bare))
	spec, _ := runWith(t, sf, time.Minute, skillsMarketplace(other.bare, "", "dev"))
	assert.Equal(t, "skills", spec.Env[vm.MarketplacesEnv])
	head := gitIn(t, other.work, "rev-parse", "HEAD")
	assert.Contains(t, sf.f.notes.String(), "marketplace skills at "+head)
}

func TestARepoFileNarrowsThePluginsAndNamesTheOnesTheDeviceLacks(t *testing.T) {
	repo := newMarketplaceRepo(t)
	sf := newSessionFixture(t)
	checkout := t.TempDir()
	gitIn(t, checkout, "init", "-q", "-b", "main")
	require.NoError(t, os.MkdirAll(filepath.Join(checkout, ".wardcase"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(checkout, ".wardcase", "config.toml"),
		[]byte(`plugins = ["skills/other", "skills/missing"]`), 0o600))
	gitIn(t, checkout, "add", ".")
	gitIn(t, checkout, "commit", "-q", "-m", "init")

	sf.f.r.Now = nil
	sf.f.box.act = func(int, run, io.Writer) int { return 0 }
	req := sf.request("agent/narrow")
	req.Source = ward.Source{
		URL: checkout, Base: "main", Branch: "agent/narrow", Token: "",
	}
	req.Marketplaces = []ward.MarketplaceConfig{
		skillsMarketplace(repo.bare, "", "dev", "other"),
	}
	_, err := sf.f.r.RunSession(context.Background(), req)
	require.NoError(t, err)

	env := sf.f.box.runs[0].spec.Env
	assert.Equal(t, "other@skills", env[vm.PluginsEnv])
	assert.Contains(t, sf.f.notes.String(), "plugin skills/missing, which the device")
}

func TestAMissingPluginIsNamedAndARemoteOneGivesNoSkillPath(t *testing.T) {
	repo := newMarketplaceRepo(t)
	sf := newSessionFixture(t)
	mk := skillsMarketplace(repo.bare, "", "dev", "remote", "ghost")
	spec, _ := runWith(t, sf, 0, mk)

	assert.Equal(t, "dev@skills remote@skills", spec.Env[vm.PluginsEnv])
	assert.Equal(t, devSkills, spec.Env[vm.SkillPathsEnv])
	assert.Contains(t, sf.f.notes.String(), "has no plugin ghost")
}

func TestAMarketplaceWithoutAManifestGivesNothing(t *testing.T) {
	bare, work := t.TempDir(), t.TempDir()
	gitIn(t, bare, "init", "-q", "--bare", "-b", "main")
	gitIn(t, work, "init", "-q", "-b", "main")
	write(t, work, "README")
	gitIn(t, work, "add", ".")
	gitIn(t, work, "commit", "-q", "-m", "init")
	gitIn(t, work, "push", "-q", bare, "HEAD:refs/heads/main")
	sf := newSessionFixture(t)

	spec, _ := runWith(t, sf, 0, skillsMarketplace(bare, "", "dev"))
	assert.NotContains(t, spec.Env, vm.MarketplacesEnv)
	assert.Contains(t, sf.f.notes.String(), "has no .claude-plugin/marketplace.json")
}

func TestAMarketplaceWithAnUnreadableManifestGivesNothing(t *testing.T) {
	bare, work := t.TempDir(), t.TempDir()
	gitIn(t, bare, "init", "-q", "--bare", "-b", "main")
	gitIn(t, work, "init", "-q", "-b", "main")
	require.NoError(t, os.MkdirAll(filepath.Join(work, ".claude-plugin"), 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(work, ".claude-plugin", "marketplace.json"), []byte("{"), 0o600))
	gitIn(t, work, "add", ".")
	gitIn(t, work, "commit", "-q", "-m", "init")
	gitIn(t, work, "push", "-q", bare, "HEAD:refs/heads/main")
	sf := newSessionFixture(t)

	spec, _ := runWith(t, sf, 0, skillsMarketplace(bare, "", "dev"))
	assert.NotContains(t, spec.Env, vm.MarketplacesEnv)
	assert.Contains(t, sf.f.notes.String(), "unreadable")
}

func TestARepoPluginListThatIsInvalidStopsTheRun(t *testing.T) {
	repo := newMarketplaceRepo(t)
	sf := newSessionFixture(t)
	checkout := t.TempDir()
	gitIn(t, checkout, "init", "-q", "-b", "main")
	require.NoError(t, os.MkdirAll(filepath.Join(checkout, ".wardcase"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(checkout, ".wardcase", "config.toml"),
		[]byte(`plugins = ["nope"]`), 0o600))
	gitIn(t, checkout, "add", ".")
	gitIn(t, checkout, "commit", "-q", "-m", "init")
	req := sf.request("agent/bad")
	req.Source = ward.Source{URL: checkout, Base: "main", Branch: "agent/bad", Token: ""}
	req.Marketplaces = []ward.MarketplaceConfig{skillsMarketplace(repo.bare, "", "dev")}

	_, err := sf.f.r.RunSession(context.Background(), req)
	assert.ErrorContains(t, err, "marketplace/plugin")
}

func TestTheDeviceConfigListsMarketplaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(`
[[marketplace]]
name = "skills"
url = "https://github.com/xdoubleu/skills"
plugins = ["dev", "task-lifecycle"]

[[marketplace]]
name = "pinned"
url = "git@github.com:a/b.git"
ref = "v1.2.0"
`), 0o600))
	cfg, err := ward.LoadConfig(path)
	require.NoError(t, err)
	require.Len(t, cfg.Marketplace, 2)
	assert.Equal(t, []string{"dev", "task-lifecycle"}, cfg.Marketplace[0].Plugins)
	assert.Equal(t, "v1.2.0", cfg.Marketplace[1].Ref)
}

func TestAnInvalidMarketplaceIsRefused(t *testing.T) {
	for name, body := range map[string]string{
		"bad name":    "name = \"Skills\"\nurl = \"https://x/y\"\n",
		"no url":      "name = \"a\"\n",
		"option url":  "name = \"a\"\nurl = \"--upload-pack=x\"\n",
		"password":    "name = \"a\"\nurl = \"https://u:p@host/y\"\n",
		"bad ref":     "name = \"a\"\nurl = \"https://x/y\"\nref = \"-x\"\n",
		"bad plugin":  "name = \"a\"\nurl = \"https://x/y\"\nplugins = [\"a b\"]\n",
		"unknown key": "name = \"a\"\nurl = \"https://x/y\"\nbranch = \"main\"\n",
		"listed twice": "name = \"a\"\nurl = \"https://x/y\"\n" +
			"[[marketplace]]\nname = \"a\"\nurl = \"https://x/z\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			require.NoError(t, os.WriteFile(path, []byte("[[marketplace]]\n"+body), 0o600))
			_, err := ward.LoadConfig(path)
			assert.Error(t, err)
		})
	}
}

func TestAChatGetsTheMarketplacesToo(t *testing.T) {
	repo := newMarketplaceRepo(t)
	sf, _ := chatFixture(t)
	sf.f.r.Config.Marketplace = []ward.MarketplaceConfig{
		skillsMarketplace(repo.bare, "", "dev"),
	}
	chat, err := sf.f.r.OpenChat(context.Background(), chatRequest(sf, newEvents()))
	require.NoError(t, err)
	defer chat.Close(context.Background())

	spec := sf.f.box.runs[0].spec
	assert.Equal(t, "dev@skills", spec.Env[vm.PluginsEnv])
	assert.Equal(t, devSkills, spec.Env[vm.SkillPathsEnv])
}
