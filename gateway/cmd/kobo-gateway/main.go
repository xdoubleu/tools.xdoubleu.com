// Command kobo-gateway is the local macOS bridge between the tools web app
// and a USB-mounted Kobo e-reader. It exposes a loopback-only HTTP API that
// the books page drives; it stores no credentials. See internal/kobogateway.
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"

	"tools.xdoubleu.com/gateway/internal/kobogateway"
)

//nolint:gochecknoglobals //Release is set at build time via -ldflags.
var Release = "dev"

// SentryDSN is set at build time via -ldflags (see Makefile), mirroring
// Release. Left empty in dev builds and go test, in which case initSentry
// skips sentry.Init entirely — nothing is ever reported off this Mac
// without an explicit DSN baked in at build time.
//
//nolint:gochecknoglobals // ldflags injection point, mirrors Release above.
var SentryDSN = ""

// headless skips the real AppKit menu bar and login-item registration —
// set by TestMain in main_test.go. There's no window server session under
// go test, so running the real menu bar would crash or hang the test run,
// and login-item registration would touch the real ~/Library/LaunchAgents.
//
//nolint:gochecknoglobals // test seam, see main_test.go's TestMain
var headless = false

const (
	readTimeout = 5 * time.Second
	// writeTimeout covers POST /update, which downloads the new binary
	// inside the handler.
	writeTimeout    = 2 * time.Minute
	shutdownTimeout = 5 * time.Second
	// koboPollInterval is how often the menu bar checks for a Kobo being
	// connected/disconnected (see kobogateway.Watch).
	koboPollInterval = 2 * time.Second
)

func main() {
	// The menu bar's AppKit run loop must run on the main OS thread.
	runtime.LockOSThread()

	initSentry()
	defer reportAndRepanic()

	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// initSentry initializes crash reporting when SentryDSN was set at build
// time (see the Makefile). A Sentry DSN is a publishable, send-only key, so
// baking it into the distributed binary is the standard approach for a
// client app — the reverse (an empty DSN, e.g. `make build` without one, or
// go test) leaves Sentry fully disabled so nothing is ever sent off the
// user's Mac. Scope is Go panics only: the darwinkit ObjC bridge's SIGABRT
// bypasses Go's panic machinery entirely and never reaches this SDK; that
// class is covered by launchd's KeepAlive relaunch, not by reporting.
func initSentry() {
	if SentryDSN == "" {
		return
	}

	environment := "production"
	if Release == "dev" {
		environment = "dev"
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:         SentryDSN,
		Release:     Release,
		Environment: environment,
		// This app is loopback-only and stores no credentials; keep the
		// crash payload to stack + release, not the user's hostname.
		ServerName: "",
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not initialize Sentry:", err)
	}
}

type stringsFlag []string

func (s *stringsFlag) String() string { return strings.Join(*s, ", ") }

func (s *stringsFlag) Set(value string) error {
	*s = append(*s, value)

	return nil
}

func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("kobo-gateway", flag.ContinueOnError)
	flags.SetOutput(stdout)
	port := flags.Int(
		"port",
		kobogateway.DefaultPort,
		"port to listen on (bound to 127.0.0.1 only)",
	)
	volumesRoot := flags.String(
		"volumes-root",
		kobogateway.DefaultVolumesRoot,
		"directory scanned for mounted Kobo volumes",
	)
	origin := flags.String(
		"origin",
		kobogateway.DefaultWebOrigin,
		"web origin the update subcommand downloads from",
	)
	var extraOrigins stringsFlag
	flags.Var(
		&extraOrigins,
		"allow-origin",
		"additional allowed web origin (repeatable)",
	)

	if err := flags.Parse(args); err != nil {
		return err
	}

	updater := kobogateway.NewUpdater()

	if flags.NArg() > 0 {
		if flags.Arg(0) != "update" {
			return fmt.Errorf("unknown command %q", flags.Arg(0))
		}

		return update(updater, *origin, stdout)
	}

	cfg := kobogateway.Config{
		Port: *port,
		AllowedOrigins: append(
			kobogateway.DefaultAllowedOrigins(),
			extraOrigins...),
		VolumesRoot: *volumesRoot,
		Release:     Release,
	}

	return serve(kobogateway.NewServer(cfg, updater), cfg, stdout)
}

func update(updater *kobogateway.Updater, origin string, stdout io.Writer) error {
	fmt.Fprintf(stdout, "downloading latest gateway from %s…\n", origin)

	if err := updater.SelfUpdate(context.Background(), origin); err != nil {
		return err
	}

	fmt.Fprintln(stdout, "updated; restart the gateway to run the new version")

	return nil
}

// certDir returns where the gateway's self-signed TLS cert/key and trust
// marker are persisted across runs (~/Library/Application Support/kobo-gateway
// on macOS).
func certDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(base, "kobo-gateway"), nil
}

// serve runs the gateway and its menu-bar UI on the main thread until it
// fails, the user quits from the menu, or a successful self-update asks for
// a restart (in which case it re-execs the freshly replaced binary).
func serve(
	gateway *kobogateway.Server,
	cfg kobogateway.Config,
	stdout io.Writer,
) error {
	certsDir, err := certDir()
	if err != nil {
		return fmt.Errorf("resolve cert dir: %w", err)
	}

	cert, certPath, err := kobogateway.EnsureCert(certsDir)
	if err != nil {
		return fmt.Errorf("prepare TLS cert: %w", err)
	}

	if err = kobogateway.EnsureTrusted(certsDir, certPath, stdout); err != nil {
		fmt.Fprintf(stdout, "warning: could not trust gateway cert automatically: %v\n", err)
		fmt.Fprintln(
			stdout,
			"open Keychain Access and trust", certPath, "manually if Safari can't reach the gateway",
		)
	}

	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      gateway.Handler(),
		TLSConfig:    &tls.Config{Certificates: []tls.Certificate{cert}},
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}

	fmt.Fprintf(
		stdout,
		"kobo-gateway %s (protocol v%d) listening on https://%s\n",
		Release,
		kobogateway.GatewayVersion,
		addr,
	)
	fmt.Fprintln(stdout, "look for the Kobo icon in the menu bar")

	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServeTLS("", "") }()

	// stop unblocks runUI (Quit menu item, server failure, or self-update).
	// serveErr/restarting are written here before stop closes and read in
	// this goroutine only after runUI returns, so the channel close is the
	// only synchronization needed (happens-before via Go's memory model).
	stop := make(chan struct{})
	var serveErr error
	var restarting bool
	go func() {
		select {
		case serveErr = <-errCh:
		case <-gateway.Restart():
			restarting = true
		}
		close(stop)
	}()

	// Never spin up the real AppKit menu bar from a test binary — it has no
	// window server session and would crash or hang the test run. This also
	// keeps login-item registration (which touches the real
	// ~/Library/LaunchAgents) out of `go test` entirely.
	watchCtx, cancelWatch := context.WithCancel(context.Background())
	defer cancelWatch()
	koboEvents := kobogateway.Watch(watchCtx, cfg.VolumesRoot, koboPollInterval)

	if headless {
		<-stop
	} else {
		homeDir, _ := os.UserHomeDir()
		execPath, _ := os.Executable()

		if homeDir != "" && execPath != "" {
			err = kobogateway.EnsureInitialLoginItem(certsDir, homeDir, execPath)
			if err != nil {
				fmt.Fprintln(stdout, "warning: could not register login item:", err)
			}

			if err = kobogateway.SyncLoginItem(homeDir, execPath); err != nil {
				fmt.Fprintln(stdout, "warning: could not refresh login item:", err)
			}
		}

		runUI(cfg.Release, stop, koboEvents, homeDir, execPath)
	}

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		shutdownTimeout,
	)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)

	if serveErr != nil {
		return serveErr
	}
	if !restarting {
		return nil
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not resolve executable to restart: %w", err)
	}

	fmt.Fprintln(stdout, "updated, restarting into the new binary…")

	//nolint:gosec //re-execs our own path as reported by os.Executable
	return syscall.Exec(executable, os.Args, os.Environ())
}
