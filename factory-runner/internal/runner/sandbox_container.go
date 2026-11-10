package runner

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/netip"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// ContainerVersion is the Apple `container` release the flags below were checked
// against (adr-0043); another major.minor is refused, as the CLI is young.
const ContainerVersion = "1.6"

const (
	containerCLI = "container"
	mountFlag    = "--mount"

	// sandboxUser is the non-root user the job runs as.
	sandboxUser = "1000:1000"
	// sandboxHome is a writable home on the tmpfs, as the root is read-only.
	sandboxHome = "/tmp/home"
	// sandboxGoCache and sandboxNPMCache are the read-only registry caches.
	sandboxGoCache  = "/gomodcache"
	sandboxNPMCache = "/npmcache"

	subnetFirst    = 7
	subnetLast     = 250
	subnetAttempts = 5
	cleanupTimeout = 30 * time.Second
	envFilePerm    = 0o600
	envDirPerm     = 0o700
)

// subnetPools are the /16s a job network is taken from, in order, so a VPN that
// holds one (10/8 is common) still leaves another.
//
//nolint:gochecknoglobals // a constant table
var subnetPools = []string{"10.231.%d.0/24", "172.29.%d.0/24", "192.168.%d.0/24"}

// envKeyPattern is a name `container --env-file` reads as a variable.
var envKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ProxyListener serves the egress proxy on an address and reports where it bound.
type ProxyListener interface {
	Listen(gateway string, port int) (stop func(), addr string, err error)
}

// Execer runs a host command; ContainerSandbox's only way to touch the host, so
// tests replace it. It returns the exit code, and an error only if the command
// could not run at all.
type Execer interface {
	Exec(ctx context.Context, argv []string, stdin io.Reader, out io.Writer) (int, error)
}

// defaultBinDirs are where Homebrew and the installers put tools; launchd's minimal
// PATH omits them, so the LaunchAgent cannot find `container` without them.
//
//nolint:gochecknoglobals // a constant table
var defaultBinDirs = []string{"/opt/homebrew/bin", "/usr/local/bin"}

// OSExecer runs commands with os/exec.
type OSExecer struct {
	// BinDirs are searched for a command that is not on PATH; empty means the
	// Homebrew and /usr/local locations.
	BinDirs []string
}

// resolve returns the command's path from PATH, else from BinDirs, else name as
// given so the caller's error names it.
func (e OSExecer) resolve(name string) string {
	if p, err := exec.LookPath(name); err == nil || strings.Contains(name, "/") {
		return cmp.Or(p, name)
	}
	dirs := e.BinDirs
	if len(dirs) == 0 {
		dirs = defaultBinDirs
	}
	for _, dir := range dirs {
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() &&
			info.Mode()&0o111 != 0 {
			return p
		}
	}
	return name
}

// Exec implements Execer.
func (e OSExecer) Exec(
	ctx context.Context,
	argv []string,
	stdin io.Reader,
	out io.Writer,
) (int, error) {
	//nolint:gosec // fixed argv
	cmd := exec.CommandContext(ctx, e.resolve(argv[0]), argv[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = out
	cmd.Stderr = out
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	if err != nil {
		return -1, fmt.Errorf("%s: %w", argv[0], err)
	}
	return 0, nil
}

// ContainerSandbox runs each job in its own Apple `container` VM on its own
// host-only network (docs/adr-0043). The VM starts as a placeholder so the host side
// of its network exists, Proxy is served there, and then the job runs in it with
// `container exec`. The job gets the gateway address as FACTORY_GATEWAY and the
// proxy's bound address as FACTORY_PROXY.
type ContainerSandbox struct {
	// Image is the job image, built from the Containerfile.
	Image string
	Exec  Execer
	// Proxy, when set, is served on the network's gateway at ProxyPort once the VM
	// runs, and stopped when the job ends. An error ends the job before the agent
	// starts. The one place that listens is also the one that tells the VM where.
	Proxy     ProxyListener
	ProxyPort int
	// GoModCache and NPMCache, when set, are mounted read-only: the VM has no
	// registry access, so the host fills them from the lockfiles.
	GoModCache string
	NPMCache   string
	CPUs       string
	Memory     string
	// TempDir holds the per-job env file; empty means os.TempDir().
	TempDir string

	// mu covers choosing a subnet through the VM starting: a network's route
	// exists only once its VM does, so a second job must wait to see it.
	mu sync.Mutex
}

// Check reports whether the `container` CLI is installed at the supported release.
func (s *ContainerSandbox) Check(ctx context.Context) error {
	var out bytes.Buffer
	code, err := s.Exec.Exec(ctx, []string{containerCLI, "--version"}, nil, &out)
	if err != nil || code != 0 {
		return fmt.Errorf("the container CLI is not usable: %w", errors.Join(err,
			fmt.Errorf("exit %d: %s", code, strings.TrimSpace(out.String()))))
	}
	if !strings.Contains(out.String(), "version "+ContainerVersion+".") {
		return fmt.Errorf("container %s.x is required, found: %s",
			ContainerVersion, strings.TrimSpace(out.String()))
	}
	return nil
}

// Run implements Sandbox.
func (s *ContainerSandbox) Run(ctx context.Context, spec SandboxSpec) (int, error) {
	if err := checkSpec(spec); err != nil {
		return -1, err
	}
	name := "factory-job-" + shortID(spec.Env["FACTORY_JOB"])
	// A crashed runner may have left these behind.
	s.cleanup(name)
	defer s.cleanup(name)

	subnet, err := s.provision(ctx, name, spec)
	if err != nil {
		return -1, err
	}
	gateway := gatewayOf(subnet)

	proxyURL := ""
	if proxy := cmp.Or(spec.Proxy, s.Proxy); proxy != nil {
		stop, addr, listenErr := proxy.Listen(gateway, s.ProxyPort)
		if listenErr != nil {
			return -1, fmt.Errorf("starting the egress proxy: %w", listenErr)
		}
		defer stop()
		proxyURL = "http://" + addr
	}

	envFile, err := s.writeEnvFile(spec.Env, gateway, proxyURL)
	if err != nil {
		return -1, err
	}
	defer os.Remove(envFile)

	return s.Exec.Exec(ctx, execArgv(name, envFile, spec), strings.NewReader(spec.Stdin),
		spec.Log)
}

// checkSpec refuses a spec the VM's command line or env file could not carry
// faithfully, before anything is started.
func checkSpec(spec SandboxSpec) error {
	// The VM and its network are named after the job, and two jobs without a name
	// would collide.
	if spec.Env["FACTORY_JOB"] == "" {
		return errors.New("the job has no FACTORY_JOB id")
	}
	for k, v := range spec.Env {
		if k == "FACTORY_GATEWAY" || k == "FACTORY_PROXY" {
			return fmt.Errorf("environment variable %q is set by the sandbox", k)
		}
		if !envKeyPattern.MatchString(k) {
			return fmt.Errorf("environment variable name %q is not valid", k)
		}
		if strings.ContainsAny(v, "\n\r") {
			return fmt.Errorf("environment variable %q has a line break", k)
		}
	}
	// A comma would add mount options to the comma-separated --mount value.
	for _, dir := range []string{spec.CloneDir, spec.OutcomeDir, spec.SessionDir} {
		if strings.ContainsAny(dir, ",\n\r") {
			return fmt.Errorf("directory %q cannot be mounted", dir)
		}
	}
	return nil
}

// provision creates the job's host-only network and starts its idle VM. One job
// at a time chooses a subnet, so each sees the routes of the jobs before it.
func (s *ContainerSandbox) provision(
	ctx context.Context,
	name string,
	spec SandboxSpec,
) (netip.Prefix, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	subnet, err := s.createNetwork(ctx, name)
	if err != nil {
		return netip.Prefix{}, err
	}
	if err = s.start(ctx, name, spec); err != nil {
		return netip.Prefix{}, err
	}
	return subnet, nil
}

// createNetwork makes the job's host-only network on a subnet no host route
// overlaps: the default pick once collided with the Mac's LAN, and the host then sent
// the gateway's traffic to the router.
func (s *ContainerSandbox) createNetwork(
	ctx context.Context,
	name string,
) (netip.Prefix, error) {
	var routes bytes.Buffer
	if _, err := s.Exec.Exec(ctx, []string{"netstat", "-rn", "-f", "inet"}, nil,
		&routes); err != nil {
		return netip.Prefix{}, fmt.Errorf("reading host routes: %w", err)
	}
	var last string
	for _, subnet := range freeSubnets(routes.String(), subnetAttempts) {
		var out bytes.Buffer
		code, err := s.Exec.Exec(ctx, []string{
			containerCLI, "network", "create", "--internal",
			"--subnet", subnet.String(), name,
		}, nil, &out)
		if err != nil {
			return netip.Prefix{}, err
		}
		if code == 0 {
			return subnet, nil
		}
		last = strings.TrimSpace(out.String())
	}
	return netip.Prefix{}, fmt.Errorf("no free subnet for the job network: %s", last)
}

// start runs the placeholder VM: the clone, outcome and session directories are
// its only mounts, its root is read-only and it holds no capabilities.
func (s *ContainerSandbox) start(
	ctx context.Context,
	name string,
	spec SandboxSpec,
) error {
	args := []string{
		containerCLI, "run", "--detach", "--rm", "--name", name, "--network", name,
		"--read-only", "--tmpfs", "/tmp", "--cap-drop", "ALL",
		"--user", sandboxUser, "--cpus", orDefault(s.CPUs, "4"),
		"--memory", orDefault(s.Memory, "8G"),
		mountFlag, mount(spec.CloneDir, SandboxCloneDir, false),
		mountFlag, mount(spec.OutcomeDir, path.Dir(SandboxOutcomePath), false),
		mountFlag, mount(spec.SessionDir, SandboxSessionDir, false),
	}
	for _, cache := range []struct{ dir, target string }{
		{s.GoModCache, sandboxGoCache}, {s.NPMCache, sandboxNPMCache},
	} {
		if cache.dir == "" {
			continue
		}
		if strings.ContainsAny(cache.dir, ",\n\r") {
			return fmt.Errorf("directory %q cannot be mounted", cache.dir)
		}
		args = append(args, mountFlag, mount(cache.dir, cache.target, true))
	}
	args = append(args, "--entrypoint", "sleep", s.Image, "infinity")

	var out bytes.Buffer
	code, err := s.Exec.Exec(ctx, args, nil, &out)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("starting the VM: %s", strings.TrimSpace(out.String()))
	}
	return nil
}

// execArgv runs the job's setup commands and then its command in the VM. The
// environment goes in a file, never argv, which any process on the Mac can read.
// Setup reads no stdin, so it cannot swallow the prompt meant for the command.
func execArgv(name, envFile string, spec SandboxSpec) []string {
	// The clone belongs to another uid on the host, so git must be told to trust it.
	script := "set -e\nmkdir -p \"$HOME\"\n" +
		"git config --global --add safe.directory " + SandboxCloneDir + "\n"
	if strings.TrimSpace(spec.SetupCommands) != "" {
		script += "(\n" + spec.SetupCommands + "\n) </dev/null\n"
	}
	script += `exec "$@"`
	argv := []string{
		containerCLI, "exec", "--interactive", "--env-file", envFile,
		"--user", sandboxUser, "--workdir", SandboxCloneDir, name, "sh", "-c", script,
		"sh",
	}
	return append(argv, spec.Command...)
}

// writeEnvFile writes the job's environment for `container exec --env-file`, with
// the fixed variables of the sandbox. The file is gone again unless it was written
// whole, so a failure leaves no half-written secrets behind.
func (s *ContainerSandbox) writeEnvFile(
	env map[string]string,
	gateway, proxyURL string,
) (string, error) {
	dir := s.TempDir
	if dir == "" {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, envDirPerm); err != nil {
		return "", fmt.Errorf("env file directory: %w", err)
	}
	file, err := os.CreateTemp(dir, "job-env-*") // 0600
	if err != nil {
		return "", fmt.Errorf("env file: %w", err)
	}
	fixed := map[string]string{
		"HOME":               sandboxHome,
		"GOMODCACHE":         sandboxGoCache,
		"GOPROXY":            "off",
		"npm_config_cache":   sandboxNPMCache,
		"npm_config_offline": "true",
	}
	var b strings.Builder
	for _, k := range slices.Sorted(maps.Keys(fixed)) {
		if _, set := env[k]; !set {
			fmt.Fprintf(&b, "%s=%s\n", k, fixed[k])
		}
	}
	fmt.Fprintf(&b, "FACTORY_GATEWAY=%s\n", gateway)
	if proxyURL != "" {
		fmt.Fprintf(&b, "FACTORY_PROXY=%s\n", proxyURL)
	}
	for _, k := range slices.Sorted(maps.Keys(env)) {
		fmt.Fprintf(&b, "%s=%s\n", k, env[k])
	}
	_, werr := file.WriteString(b.String())
	cerr := file.Close()
	if err = errors.Join(werr, cerr); err != nil {
		_ = os.Remove(file.Name())
		return "", fmt.Errorf("env file: %w", err)
	}
	return file.Name(), nil
}

// cleanup kills the VM and removes it and its network, both named after the job. It
// is best effort and runs on its own context, so a cancelled job is still cleaned
// up. Kill, not stop: the idle VM's `sleep` ignores SIGTERM as PID 1 and stop would
// wait out its grace period.
func (s *ContainerSandbox) cleanup(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	var sink bytes.Buffer
	_, _ = s.Exec.Exec(ctx, []string{containerCLI, "kill", name}, nil, &sink)
	_, _ = s.Exec.Exec(ctx, []string{containerCLI, "delete", "--force", name}, nil,
		&sink)
	_, _ = s.Exec.Exec(ctx, []string{containerCLI, "network", "delete", name}, nil,
		&sink)
}

func mount(source, target string, readonly bool) string {
	m := "type=bind,source=" + filepath.Clean(source) + ",target=" + target
	if readonly {
		m += ",readonly"
	}
	return m
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// idPattern keeps the VM's name to characters `container` accepts.
var idPattern = regexp.MustCompile(`[^a-z0-9]`)

// shortID is the first characters of a job id that are safe in a name.
func shortID(id string) string {
	id = idPattern.ReplaceAllString(strings.ToLower(id), "")
	const maxLen = 12
	if len(id) > maxLen {
		id = id[:maxLen]
	}
	if id == "" {
		return "x"
	}
	return id
}

// gatewayOf is the host's address on a /24 job network: its first host.
func gatewayOf(subnet netip.Prefix) string {
	return subnet.Addr().Next().String()
}

// freeSubnets lists up to n /24 job subnets, from the pools in order, that overlap
// no route in the output of `netstat -rn -f inet`.
func freeSubnets(routes string, n int) []netip.Prefix {
	var taken []netip.Prefix
	for _, line := range strings.Split(routes, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if p, ok := parseRouteDestination(fields[0]); ok {
			taken = append(taken, p)
		}
	}
	var out []netip.Prefix
	for _, pool := range subnetPools {
		for i := subnetFirst; i <= subnetLast && len(out) < n; i++ {
			cand := netip.MustParsePrefix(fmt.Sprintf(pool, i))
			if !overlapsAny(taken, cand) {
				out = append(out, cand)
			}
		}
	}
	return out
}

func overlapsAny(taken []netip.Prefix, cand netip.Prefix) bool {
	for _, t := range taken {
		if t.Overlaps(cand) {
			return true
		}
	}
	return false
}

// parseRouteDestination reads a netstat destination: "10.231.7/24", "10/8",
// "192.168.1.5" or "default" (not an IPv4 network, so it is skipped).
func parseRouteDestination(dest string) (netip.Prefix, bool) {
	addrPart, bitsPart, hasBits := strings.Cut(dest, "/")
	octets := strings.Split(addrPart, ".")
	if len(octets) == 0 || len(octets) > 4 || octets[0] == "" {
		return netip.Prefix{}, false
	}
	known := len(octets)
	for len(octets) < 4 {
		octets = append(octets, "0")
	}
	addr, err := netip.ParseAddr(strings.Join(octets, "."))
	if err != nil {
		return netip.Prefix{}, false
	}
	bits := known * 8 //nolint:mnd // a dotted prefix of n octets is n*8 bits
	if hasBits {
		if _, scanErr := fmt.Sscanf(bitsPart, "%d", &bits); scanErr != nil {
			return netip.Prefix{}, false
		}
	}
	p, err := addr.Prefix(bits)
	if err != nil {
		return netip.Prefix{}, false
	}
	return p, true
}
