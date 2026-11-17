package runner

import (
	"cmp"
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
// when it changes only the Containerfile and every instruction it adds or
// changes (whole instructions, so a package added to an existing install list
// is checked with the rest of it) is allowed and pinned.
func containerfileProblem(files []string, oldText, newText string) error {
	if len(files) != 1 || files[0] != sandboxContainerfile {
		return fmt.Errorf("it must change only %s, not %s", sandboxContainerfile,
			strings.Join(files, ", "))
	}
	had := map[string]bool{}
	for _, in := range instructions(oldText) {
		had[in] = true
	}
	for _, in := range instructions(newText) {
		if had[in] {
			continue
		}
		if err := pinnedCommand(in); err != nil {
			return err
		}
	}
	return nil
}

// instructions splits a Containerfile into logical instructions: continuation
// lines joined, comments and blank lines dropped, whitespace collapsed.
func instructions(text string) []string {
	var out []string
	var cur []string
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if len(cur) == 0 && (line == "" || strings.HasPrefix(line, "#")) {
			continue
		}
		if rest, more := strings.CutSuffix(line, "\\"); more {
			cur = append(cur, rest)
			continue
		}
		cur = append(cur, line)
		out = append(out, strings.Join(strings.Fields(strings.Join(cur, " ")), " "))
		cur = nil
	}
	if len(cur) > 0 {
		out = append(out, strings.Join(strings.Fields(strings.Join(cur, " ")), " "))
	}
	return out
}

// pinnedCommand checks one logical Containerfile instruction.
func pinnedCommand(cmd string) error {
	word, rest, _ := strings.Cut(cmd, " ")
	switch strings.ToUpper(word) {
	case "FROM":
		if !fromPinned.MatchString("FROM " + rest) {
			return fmt.Errorf("base image is not pinned to a version: %q", cmd)
		}
		return nil
	case "ARG", "ENV", "LABEL", "WORKDIR", "USER", "EXPOSE", "CMD", "ENTRYPOINT":
		return nil
	case "RUN":
	default:
		return fmt.Errorf("instruction %q is not allowed", word)
	}
	if strings.Contains(rest, "$(") || strings.Contains(rest, "`") {
		return errors.New("command substitution is not allowed")
	}
	words := strings.Fields(rest)
	for len(words) > 0 && strings.HasPrefix(words[0], "--") {
		words = words[1:]
	}
	for _, seg := range segmentSplt.Split(strings.Join(words, " "), -1) {
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
	for len(words) > 0 && (words[0] == "sudo" || words[0] == "env") {
		words = words[1:]
	}
	if len(words) == 0 {
		return nil
	}
	switch words[0] {
	case "sh", "bash", "zsh", "eval", "xargs", "source", ".":
		return fmt.Errorf("%s is not allowed", words[0])
	}
	for _, w := range words {
		if strings.HasPrefix(w, "http://") || strings.HasPrefix(w, "https://") {
			path, _, _ := strings.Cut(w, "?")
			if !semver.MatchString(path) {
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
	var oldText, newText string
	if len(files) == 1 && files[0] == sandboxContainerfile {
		o, _ := r.g.git(ctx, r.dir, "show",
			"refs/heads/"+base+":"+sandboxContainerfile)
		n, nerr := r.g.git(ctx, r.dir, "show",
			"refs/heads/"+r.branch+":"+sandboxContainerfile)
		if nerr != nil {
			return fmt.Errorf("%w: could not read the Containerfile", ErrHostCheck)
		}
		oldText, newText = string(o), string(n)
	}
	if perr := containerfileProblem(files, oldText, newText); perr != nil {
		return fmt.Errorf("%w: %w", ErrHostCheck, perr)
	}
	return nil
}

// publishSandboxFix pushes the Containerfile-only branch the outcome names and
// opens its own pull request. It returns a line for the outcome and, when the
// branch itself was refused (name, files or Containerfile), why, so the agent can
// fix it. It never fails the job: the job's own work is published either way.
func (r *JobRunner) publishSandboxFix(
	ctx context.Context,
	job *ClaimedJob,
	cloneDir, text string,
	secrets []string,
) (string, string) {
	name, ok := findLine(text, "Sandbox-fix:")
	if !ok {
		return "", ""
	}
	if !sandboxBranchPattern.MatchString(name) {
		why := "the branch must be named agent/sandbox-<slug>."
		return "Sandbox fix not published: " + why, why
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
		return "Sandbox fix not published: " + why, why
	}
	defer reviewed.Close()

	ctx, cancel := context.WithTimeout(ctx, publishTimeout)
	defer cancel()
	token, err := r.pushToken(ctx, job.ID)
	if err != nil || token == "" {
		return "Sandbox fix not published: could not get a push token.", ""
	}
	remote := r.Git.RemoteURL(job.RepoOwner, job.RepoName)
	if err = reviewed.Push(ctx, remote, token); err != nil {
		r.Log.Warn("sandbox fix push failed", "job", job.ID,
			"error", scrub(err.Error(), token))
		return "Sandbox fix not published: the branch could not be pushed.", ""
	}
	problem, _ := findLine(text, "Blocked:")
	body := neutralize("The sandbox lacked a tool while running task `" + job.TaskID +
		"` (job `" + job.ID + "`): " +
		cmp.Or(problem, "see the job outcome") + "\n\nThis changes only `" +
		sandboxContainerfile + "` and pins every install. Its " +
		"`factory-sandbox` board task is named in the job's outcome.")
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
			"could not be opened.", ""
	}
	return "Sandbox fix opened: " + url, ""
}

// sandboxRefusedFeedback tells the agent why its sandbox-fix branch was refused.
func sandboxRefusedFeedback(why string) string {
	return "The runner refused your sandbox fix: " + why + "\n\n" + sandboxFixRules +
		"\n\nFix the branch, commit, and write the outcome again."
}

// sandboxFixRules is what a sandbox-fix Containerfile may contain.
const sandboxFixRules = "A sandbox fix changes only factory-runner/Containerfile. " +
	"Allowed instructions: FROM (base image pinned to a version tag or digest), " +
	"ARG, ENV, LABEL, WORKDIR, USER, EXPOSE, CMD, ENTRYPOINT and RUN. COPY and ADD " +
	"are not allowed. Every package in a RUN install is pinned (apt pkg=ver, npm " +
	"name@x.y.z, pip name==x.y.z, go module@vX.Y.Z), including ones already " +
	"named in the instruction you edit (ca-certificates=ver too); a download URL " +
	"carries an explicit version; no command substitution, no piping into sh."
