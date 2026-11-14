package runner

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// sandboxContainerfile is the one file a sandbox-fix branch may change.
const (
	sandboxContainerfile = "factory-runner/Containerfile"
	npmCmd               = "npm"
)

var (
	// sandboxBranchPattern is the branch name an outcome's `Sandbox-fix:` line
	// may name.
	sandboxBranchPattern = regexp.MustCompile(`^agent/sandbox-[a-z0-9][a-z0-9-]{0,60}$`)
	// semver is an explicit version number: 1.2, 1.2.3, v1.2.3.
	semver     = regexp.MustCompile(`\d+\.\d+`)
	npmPinned  = regexp.MustCompile(`.@v?\d+\.\d+\.\d+\S*$`)
	goPinned   = regexp.MustCompile(`@v\d+\.\d+\.\d+\S*$`)
	pipPinned  = regexp.MustCompile(`==\d`)
	fromPinned = regexp.MustCompile(
		`^FROM\s+(--\S+\s+)?\S+(:[^\s:]*\d[^\s:]*|@sha256:\S+)(\s|$)`)
	segmentSplt = regexp.MustCompile(`&&|\|\||;|\|`)
)

// containerfileProblem is why a sandbox-fix diff may not be published, or nil
// when it changes only the Containerfile and every install it adds is pinned.
func containerfileProblem(files, added []string) error {
	if len(files) == 0 || len(files) > 1 || files[0] != sandboxContainerfile {
		return fmt.Errorf("it must change only %s, not %s", sandboxContainerfile,
			strings.Join(files, ", "))
	}
	var cur strings.Builder
	flush := func() error {
		cmd := strings.TrimSpace(cur.String())
		cur.Reset()
		return pinnedCommand(cmd)
	}
	for _, line := range added {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, "\\") {
			cur.WriteString(strings.TrimSuffix(line, "\\") + " ")
			continue
		}
		cur.WriteString(line)
		if err := flush(); err != nil {
			return err
		}
	}
	return flush()
}

// pinnedCommand checks one logical Containerfile instruction.
func pinnedCommand(cmd string) error {
	if cmd == "" || strings.HasPrefix(cmd, "#") {
		return nil
	}
	if strings.HasPrefix(strings.ToUpper(cmd), "FROM ") {
		if !fromPinned.MatchString(cmd) {
			return fmt.Errorf("base image is not pinned to a version: %q", cmd)
		}
		return nil
	}
	cmd = strings.TrimSpace(strings.TrimPrefix(cmd, "RUN "))
	for _, seg := range segmentSplt.Split(cmd, -1) {
		if err := pinnedSegment(strings.Fields(seg)); err != nil {
			return err
		}
	}
	return nil
}

func pinnedSegment(words []string) error {
	if len(words) == 0 {
		return nil
	}
	switch words[0] {
	case "sh", "bash", "zsh":
		return errors.New("piping into a shell is not allowed")
	}
	for _, w := range words {
		if strings.HasPrefix(w, "http://") || strings.HasPrefix(w, "https://") {
			if !semver.MatchString(w) {
				return fmt.Errorf("download %q has no explicit version", w)
			}
		}
	}
	mgr, args, isInstall := installArgs(words)
	if !isInstall {
		return nil
	}
	pkgs := 0
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		pkgs++
		if !pinnedPackage(mgr, a) {
			return fmt.Errorf("%s install %q is not pinned to a version", mgr, a)
		}
	}
	if pkgs == 0 {
		return fmt.Errorf("%s install names no package", mgr)
	}
	return nil
}

// installArgs returns the package manager and the words after `install`, and
// whether the segment installs anything. A manager it does not know is refused.
func installArgs(words []string) (string, []string, bool) {
	switch words[0] {
	case "apt-get", "apt", "pip", "pip3", "go", "gem", "cargo", "brew", "yarn",
		"pnpm", npmCmd:
	default:
		return "", nil, false
	}
	for i, w := range words[1:] {
		if w == "install" || w == "i" || w == "add" {
			return words[0], words[i+2:], true
		}
	}
	return "", nil, false
}

func pinnedPackage(mgr, pkg string) bool {
	switch mgr {
	case "apt-get", "apt":
		_, ver, ok := strings.Cut(pkg, "=")
		return ok && ver != ""
	case npmCmd, "yarn", "pnpm":
		return npmPinned.MatchString(pkg)
	case "pip", "pip3":
		return pipPinned.MatchString(pkg)
	case "go":
		return goPinned.MatchString(pkg)
	}
	return false
}

// CheckSandboxFix refuses a reviewed sandbox-fix branch whose net change from
// base is anything but pinned installs in the Containerfile.
func (r *Reviewed) CheckSandboxFix(ctx context.Context, base string) error {
	net := "refs/heads/" + base + "...refs/heads/" + r.branch
	out, err := r.g.git(ctx, r.dir, "diff", "--name-only", "--no-renames",
		"--no-ext-diff", net)
	if err != nil {
		return fmt.Errorf("%w: could not list changed files: %w", ErrHostCheck, err)
	}
	var files []string
	for f := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if f != "" {
			files = append(files, f)
		}
	}
	if len(files) == 1 && files[0] == sandboxContainerfile {
		out, err = r.g.git(ctx, r.dir, "diff", "-U0", "--no-renames", "--no-ext-diff", net)
		if err != nil {
			return fmt.Errorf("%w: could not read the diff: %w", ErrHostCheck, err)
		}
	} else {
		out = nil
	}
	var added []string
	for l := range strings.SplitSeq(string(out), "\n") {
		if strings.HasPrefix(l, "+") && !strings.HasPrefix(l, "+++") {
			added = append(added, l[1:])
		}
	}
	if perr := containerfileProblem(files, added); perr != nil {
		return fmt.Errorf("%w: %w", ErrHostCheck, perr)
	}
	return nil
}

// publishSandboxFix pushes the Containerfile-only branch the outcome names and
// opens its own pull request. It returns a line for the outcome and never fails
// the job: the job's own work is published either way.
func (r *JobRunner) publishSandboxFix(
	ctx context.Context,
	job *ClaimedJob,
	cloneDir, text string,
	secrets []string,
) string {
	name, ok := findLine(text, "Sandbox-fix:")
	if !ok {
		return ""
	}
	if !sandboxBranchPattern.MatchString(name) {
		return "Sandbox fix not published: the branch must be named agent/sandbox-<slug>."
	}
	reviewed, err := r.Git.Inspect(ctx, job.RepoOwner, job.RepoName, job.DefaultBranch,
		name, job.ID+"-sandbox", cloneDir, "", secrets)
	if err == nil {
		err = reviewed.CheckSandboxFix(ctx, job.DefaultBranch)
	}
	if err != nil {
		if reviewed != nil {
			reviewed.Close()
		}
		why := strings.TrimPrefix(err.Error(), ErrHostCheck.Error()+": ")
		return "Sandbox fix not published: " + why
	}
	defer reviewed.Close()

	ctx, cancel := context.WithTimeout(ctx, publishTimeout)
	defer cancel()
	token, err := r.pushToken(ctx, job.ID)
	if err != nil || token == "" {
		return "Sandbox fix not published: could not get a push token."
	}
	remote := r.Git.RemoteURL(job.RepoOwner, job.RepoName)
	if err = reviewed.Push(ctx, remote, token); err != nil {
		r.Log.Warn("sandbox fix push failed", "job", job.ID,
			"error", scrub(err.Error(), token))
		return "Sandbox fix not published: the branch could not be pushed."
	}
	problem, _ := findLine(text, "Blocked:")
	body := neutralize("The sandbox lacked a tool while running task `" + job.TaskID +
		"` (job `" + job.ID + "`): " + problem + "\n\nThis changes only `" +
		sandboxContainerfile + "` and pins every install. Linked board task: the " +
		"`factory-sandbox` task named in that job's outcome.")
	url, err := r.openPullRequest(ctx, token, PullRequest{
		Owner: job.RepoOwner, Repo: job.RepoName, Head: name, Base: job.DefaultBranch,
		Title: "Sandbox: " + strings.ReplaceAll(
			strings.TrimPrefix(name, "agent/sandbox-"), "-", " "),
		Body: body,
	})
	if err != nil {
		r.Log.Warn("sandbox fix pull request failed", "job", job.ID,
			"error", scrub(err.Error(), token))
		return "Sandbox fix not published: the branch was pushed, but its pull request " +
			"could not be opened."
	}
	return "Sandbox fix opened: " + url
}
