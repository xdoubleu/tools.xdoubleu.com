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

	"tools.xdoubleu.com/kobo-gateway/internal/kobogateway"
)

//nolint:gochecknoglobals //Release is set at build time via -ldflags.
var Release = "dev"

// SentryDSN is set via -ldflags; empty disables Sentry.
//
//nolint:gochecknoglobals // ldflags injection point, mirrors Release above.
var SentryDSN = ""

// headless skips the AppKit menu bar and login-item registration under go
// test (no window server; would touch ~/Library/LaunchAgents).
//
//nolint:gochecknoglobals // test seam, see main_test.go's TestMain
var headless = false

// restarting is set before stop closes when self-update asked for a restart;
// read by menubar_darwin.go after <-stop. serve() resets it on entry.
//
//nolint:gochecknoglobals // cross-file signal between serve and runUI, see above.
var restarting bool

const (
	readTimeout = 5 * time.Second
	// writeTimeout covers POST /update, which downloads inside the handler.
	writeTimeout     = 2 * time.Minute
	shutdownTimeout  = 5 * time.Second
	koboPollInterval = 2 * time.Second
)

func main() {
	// The menu bar's AppKit run loop must run on the main OS thread.
	runtime.LockOSThread()

	initSentry()
	defer reportAndRepanic()

	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		// The deferred reportAndRepanic covers panics only.
		reportFatal(err)
		os.Exit(1) //nolint:gocritic //reportFatal already reported this error, see above
	}
}

// initSentry enables crash reporting when SentryDSN is set (a publishable,
// send-only key). Covers Go panics only: the darwinkit bridge's SIGABRT
// bypasses Go and is handled by launchd's KeepAlive relaunch.
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
		// Loopback-only, no credentials: don't send the hostname.
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

	gateway := kobogateway.NewServer(cfg, updater)
	gateway.SetNotifier(notify)

	return serve(gateway, cfg, stdout)
}

func update(updater *kobogateway.Updater, origin string, stdout io.Writer) error {
	fmt.Fprintf(stdout, "downloading latest gateway from %s…\n", origin)

	if err := updater.SelfUpdate(context.Background(), origin); err != nil {
		return err
	}

	fmt.Fprintln(stdout, "updated; restart the gateway to run the new version")

	return nil
}

// certDir returns where the TLS cert/key and trust marker persist.
func certDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(base, "kobo-gateway"), nil
}

// serve runs the gateway and menu-bar UI until failure, quit, or a
// self-update restart (which re-execs the replaced binary).
//
//nolint:funlen
func serve(
	gateway *kobogateway.Server,
	cfg kobogateway.Config,
	stdout io.Writer,
) error {
	restarting = false

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
			"open Keychain Access and trust", certPath,
			"manually if Safari can't reach the gateway",
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

	// serveErr/restarting are written before stop closes and read after runUI
	// returns; the close is the synchronization.
	stop := make(chan struct{})
	var serveErr error
	go func() {
		select {
		case serveErr = <-errCh:
		case <-gateway.Restart():
			restarting = true

			// Release the port before runUI relaunches via `open -n`: the new process
			// binds the same port and exits if it's taken. The /update response was
			// already written.
			shutdownCtx, cancel := context.WithTimeout(
				context.Background(),
				shutdownTimeout,
			)
			_ = server.Shutdown(shutdownCtx)
			cancel()
		}
		close(stop)
	}()

	// Never run the real menu bar or login-item registration under go test.
	watchCtx, cancelWatch := context.WithCancel(context.Background())
	defer cancelWatch()
	koboEvents := kobogateway.Watch(watchCtx, cfg.VolumesRoot, koboPollInterval)

	// Read before EnsureInitialLoginItem creates the marker, and before the
	// headless branch so tests cover it.
	firstLaunch := kobogateway.IsFirstLaunch(certsDir)

	//nolint:nestif //extracting this only relocates coverage gaps, see git history
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

		runUI(cfg.Release, stop, koboEvents, homeDir, execPath, firstLaunch)
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
