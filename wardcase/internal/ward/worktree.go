package ward

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	worktreesDir = "worktrees"
	branchPrefix = "wardcase/"
	// baseConfig records the commit a session's branch was cut from, so commits
	// the caller already had are not counted as the session's.
	baseConfig = "wardcaseBase"
	// pathParts is <checkout-hash>/worktrees/<id>.
	pathParts = 3
)

// session is the worktree one run works in; the zero value is the caller's own
// directory, which wardcase leaves alone.
type session struct {
	// path is the worktree; empty when running in place.
	path   string
	branch string
	// home is the caller's checkout, where git runs to remove the worktree.
	home string
}

func none() session { return session{path: "", branch: "", home: ""} }

// resumeRequested says the harness arguments continue an earlier session.
func resumeRequested(args []string) bool {
	return slices.ContainsFunc(args, func(a string) bool {
		name, _, _ := strings.Cut(a, "=")
		switch name {
		case "--resume", "-r", "--continue", "-c", "--session", "-s", "resume":
			return true
		}
		return false
	})
}

// toplevel is the repository's top level for dir.
func (r *Runner) toplevel(ctx context.Context, dir string) (string, bool) {
	top, err := r.Git(ctx, dir, "rev-parse", "--show-toplevel")
	top = strings.TrimSpace(top)
	return top, err == nil && top != ""
}

// openSession gives the run its worktree: the one the caller is standing in when
// it is wardcase's, the checkout's latest on a resume, else a new branch cut from
// the caller's HEAD.
func (r *Runner) openSession(ctx context.Context, req Request) (session, error) {
	top, ok := r.toplevel(ctx, req.Dir)
	if !ok || req.InPlace {
		return none(), nil
	}
	if s, found := r.ownWorktree(ctx, top); found {
		return s, nil
	}
	if resumeRequested(req.Args) {
		if s, found := r.latestWorktree(ctx, top); found {
			return s, nil
		}
	}
	return r.newWorktree(ctx, top)
}

func (r *Runner) checkoutDir(top string) string {
	return filepath.Dir(r.sessionDir(top, "x"))
}

// ownWorktree is top when it is one of wardcase's worktrees.
func (r *Runner) ownWorktree(ctx context.Context, top string) (session, bool) {
	rel, err := filepath.Rel(r.StateDir, top)
	if err != nil {
		return none(), false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) != pathParts || parts[1] != worktreesDir {
		return none(), false
	}
	return r.reuse(ctx, top)
}

func (r *Runner) latestWorktree(ctx context.Context, top string) (session, bool) {
	dir := filepath.Join(r.checkoutDir(top), worktreesDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return none(), false
	}
	var best string
	var bestTime int64
	for _, e := range entries {
		info, infoErr := e.Info()
		if infoErr != nil || !e.IsDir() {
			continue
		}
		if t := info.ModTime().UnixNano(); best == "" || t > bestTime {
			best, bestTime = filepath.Join(dir, e.Name()), t
		}
	}
	if best == "" {
		return none(), false
	}
	s, ok := r.reuse(ctx, best)
	return s, ok
}

func (r *Runner) reuse(ctx context.Context, path string) (session, bool) {
	branch, err := r.Git(ctx, path, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return none(), false
	}
	now := r.now()
	_ = os.Chtimes(path, now, now)
	home := path
	common, err := r.Git(ctx, path, "rev-parse", "--path-format=absolute",
		"--git-common-dir")
	if c := strings.TrimSpace(common); err == nil && c != "" {
		home = c
	}
	return session{path: path, branch: strings.TrimSpace(branch), home: home}, true
}

func (r *Runner) newWorktree(ctx context.Context, top string) (session, error) {
	id := randomID()
	path := filepath.Join(r.checkoutDir(top), worktreesDir, id)
	branch := branchPrefix + id
	if err := os.MkdirAll(filepath.Dir(path), stateDirPerm); err != nil {
		return none(), fmt.Errorf("worktree directory: %w", err)
	}
	base, err := r.Git(ctx, top, "rev-parse", "HEAD")
	if err != nil {
		return none(), fmt.Errorf("the checkout has no commit to branch from: %w", err)
	}
	_, err = r.Git(ctx, top, "worktree", "add", "-b", branch, path, "HEAD")
	if err != nil {
		return none(), fmt.Errorf("create the worktree: %w", err)
	}
	_, _ = r.Git(ctx, top, "config", "branch."+branch+"."+baseConfig,
		strings.TrimSpace(base))
	fmt.Fprintf(r.Notes, "wardcase: working in %s on branch %s\n", path, branch)
	return session{path: path, branch: branch, home: top}, nil
}

// left is what a session's worktree holds that is not yet shipped.
type left struct {
	uncommitted int
	unpushed    int
	// own counts the commits the session added to its base; -1 when unknown.
	own int
	// pr is the branch's pull request state (OPEN, MERGED, CLOSED), or "" when
	// there is none or gh could not say.
	pr    string
	prURL string
}

func (l left) nothing() bool { return l.uncommitted == 0 && l.unpushed == 0 }

// finish reports what the session left in its worktree and removes the worktree
// when nothing is left and any pull request merged.
func (r *Runner) finish(ctx context.Context, s session) {
	if s.path == "" {
		return
	}
	l := r.inspect(ctx, s)
	if l.nothing() && (l.pr == "MERGED" || (l.pr == "" && l.own == 0)) {
		if r.remove(ctx, s) {
			fmt.Fprintf(r.Notes, "wardcase: nothing left in %s; removed it\n", s.path)
			return
		}
	}
	fmt.Fprintf(r.Notes, "wardcase: left in %s (branch %s):\n", s.path, s.branch)
	fmt.Fprintf(r.Notes, "  %s\n  %s\n  %s\n", plural(l.uncommitted, "uncommitted change"),
		plural(l.unpushed, "commit")+" not on any remote", l.prLine())
	fmt.Fprintf(r.Notes, "  continue with `wardcase --resume` from that directory\n")
}

func (l left) prLine() string {
	switch l.pr {
	case "":
		return "no pull request"
	default:
		return "pull request " + strings.ToLower(l.pr) + " " + l.prURL
	}
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	if n == 0 {
		return "no " + noun + "s"
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func (r *Runner) inspect(ctx context.Context, s session) left {
	l := left{uncommitted: 0, unpushed: 0, own: -1, pr: "", prURL: ""}
	status, err := r.Git(ctx, s.path, "status", "--porcelain")
	if err != nil {
		l.uncommitted = 1 // unknown counts as something left
	}
	l.uncommitted += len(nonEmptyLines(status))
	args := []string{"rev-list", "--count", "HEAD", "--not", "--remotes"}
	base, _ := r.Git(ctx, s.path, "config", "--get", "branch."+s.branch+"."+baseConfig)
	if b := strings.TrimSpace(base); b != "" {
		args = append(args, b)
		own, ownErr := r.Git(ctx, s.path, "rev-list", "--count", "HEAD", "--not", b)
		if _, scanErr := fmt.Sscan(strings.TrimSpace(own), &l.own); ownErr != nil ||
			scanErr != nil {
			l.own = -1
		}
	}
	count, err := r.Git(ctx, s.path, args...)
	if err != nil {
		l.unpushed = 1
	}
	if _, scanErr := fmt.Sscan(strings.TrimSpace(count), &l.unpushed); scanErr != nil {
		l.unpushed = 1
	}
	l.pr, l.prURL = r.pullRequest(ctx, s)
	return l
}

// pullRequest is the branch's pull request, when gh is there and knows one.
func (r *Runner) pullRequest(ctx context.Context, s session) (string, string) {
	if r.GH == nil {
		return "", ""
	}
	out, err := r.GH(ctx, s.path, "pr", "list", "--head", s.branch, "--state", "all",
		"--json", "state,url", "--limit", "1")
	if err != nil {
		return "", ""
	}
	var prs []struct {
		State string `json:"state"`
		URL   string `json:"url"`
	}
	if json.Unmarshal([]byte(out), &prs) != nil || len(prs) == 0 {
		return "", ""
	}
	return prs[0].State, prs[0].URL
}

func (r *Runner) remove(ctx context.Context, s session) bool {
	if _, err := r.Git(ctx, s.home, "worktree", "remove", s.path); err != nil {
		return false
	}
	_, _ = r.Git(ctx, s.home, "branch", "-D", s.branch)
	return true
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}
