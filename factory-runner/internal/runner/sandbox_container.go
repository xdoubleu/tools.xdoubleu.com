package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
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

// Execer runs a host command; ContainerSandbox's only way to touch the host, so
// tests replace it. It returns the exit code, and an error only if the command
// could not run at all.
type Execer interface {
	Exec(ctx context.Context, argv []string, stdin io.Reader, out io.Writer) (int, error)
}

// OSExecer runs commands with os/exec.
type OSExecer struct{}

// Exec implements Execer.
func (OSExecer) Exec(
	ctx context.Context,
	argv []string,
	stdin io.Reader,
	out io.Writer,
) (int, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // fixed argv
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
// of its network exists, OnReady can bind the egress proxy there, and then the job
// runs in it with `container exec`.
type ContainerSandbox struct {
	// Image is the job image, built from the Containerfile.
	Image string
	Exec  Execer
	// OnReady, when set, is called once the VM runs with the network's gateway
	// address, which is bindable on the host. The returned stop func runs when the
	// job ends. An error ends the job before the agent starts.
	OnReady func(ctx context.Context, gateway string) (func(), error)
	// GoModCache and NPMCache, when set, are mounted read-only: the VM has no
	// registry access, so the host fills them from the lockfiles.
	GoModCache string
	NPMCache   string
	CPUs       string
	Memory     string
	// TempDir holds the per-job env file; empty means os.TempDir().
	TempDir string

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
	name := "factory-job-" + shortID(spec.Env["FACTORY_JOB"])
	// A crashed runner may have left these behind.
	s.cleanup(name)

	subnet, err := s.createNetwork(ctx, name)
	if err != nil {
		return -1, err
	}
	defer s.cleanup(name)

	envFile, err := s.writeEnvFile(spec.Env)
	if err != nil {
		return -1, err
	}
	defer os.Remove(envFile)

	if err = s.start(ctx, name, spec); err != nil {
		return -1, err
	}

	if s.OnReady != nil {
		stop, readyErr := s.OnReady(ctx, gatewayOf(subnet))
		if readyErr != nil {
			return -1, fmt.Errorf("starting the egress proxy: %w", readyErr)
		}
		defer stop()
	}

	return s.Exec.Exec(ctx, execArgv(name, envFile, spec), strings.NewReader(spec.Stdin),
		spec.Log)
}

// createNetwork makes the job's host-only network on a subnet no host route
// overlaps: the default pick once collided with the Mac's LAN, and the host then sent
// the gateway's traffic to the router.
func (s *ContainerSandbox) createNetwork(
	ctx context.Context,
	name string,
) (netip.Prefix, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

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
	if s.GoModCache != "" {
		args = append(args, mountFlag, mount(s.GoModCache, sandboxGoCache, true))
	}
	if s.NPMCache != "" {
		args = append(args, mountFlag, mount(s.NPMCache, sandboxNPMCache, true))
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
func execArgv(name, envFile string, spec SandboxSpec) []string {
	// The clone belongs to another uid on the host, so git must be told to trust it.
	script := "set -e\nmkdir -p \"$HOME\"\n" +
		"git config --global --add safe.directory " + SandboxCloneDir + "\n"
	if strings.TrimSpace(spec.SetupCommands) != "" {
		script += spec.SetupCommands + "\n"
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
// the fixed variables of the sandbox.
func (s *ContainerSandbox) writeEnvFile(env map[string]string) (string, error) {
	dir := s.TempDir
	if dir == "" {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, envDirPerm); err != nil {
		return "", fmt.Errorf("env file directory: %w", err)
	}
	file, err := os.CreateTemp(dir, "job-env-*")
	if err != nil {
		return "", fmt.Errorf("env file: %w", err)
	}
	defer file.Close()
	if err = os.Chmod(file.Name(), envFilePerm); err != nil {
		return "", fmt.Errorf("env file mode: %w", err)
	}
	fixed := map[string]string{
		"HOME":               sandboxHome,
		"GOMODCACHE":         sandboxGoCache,
		"GOPROXY":            "off",
		"npm_config_cache":   sandboxNPMCache,
		"npm_config_offline": "true",
	}
	var b strings.Builder
	for k, v := range fixed {
		if _, set := env[k]; !set {
			fmt.Fprintf(&b, "%s=%s\n", k, v)
		}
	}
	for k, v := range env {
		if strings.ContainsAny(k+v, "\n\r") {
			return "", fmt.Errorf("environment variable %q has a line break", k)
		}
		fmt.Fprintf(&b, "%s=%s\n", k, v)
	}
	if _, err = file.WriteString(b.String()); err != nil {
		return "", fmt.Errorf("env file: %w", err)
	}
	return file.Name(), nil
}

// cleanup stops the VM and removes the network, both named after the job. It is
// best effort and runs on its own context, so a cancelled job is still cleaned up.
func (s *ContainerSandbox) cleanup(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	var sink bytes.Buffer
	_, _ = s.Exec.Exec(ctx, []string{containerCLI, "stop", name}, nil, &sink)
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

// freeSubnets lists up to n 10.231.N.0/24 subnets that overlap no route in the
// output of `netstat -rn -f inet`.
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
	for i := subnetFirst; i <= subnetLast && len(out) < n; i++ {
		cand := netip.MustParsePrefix(fmt.Sprintf("10.231.%d.0/24", i))
		overlap := false
		for _, t := range taken {
			if t.Overlaps(cand) {
				overlap = true
				break
			}
		}
		if !overlap {
			out = append(out, cand)
		}
	}
	return out
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
