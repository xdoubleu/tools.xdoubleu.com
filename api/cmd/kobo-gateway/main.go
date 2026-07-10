// Command kobo-gateway is the local macOS bridge between the tools web app
// and a USB-mounted Kobo e-reader. It exposes a loopback-only HTTP API that
// the books page drives; it stores no credentials. See internal/kobogateway.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"tools.xdoubleu.com/internal/kobogateway"
)

//nolint:gochecknoglobals //Release is set at build time via -ldflags.
var Release = "dev"

const (
	readTimeout = 5 * time.Second
	// writeTimeout covers POST /update, which downloads the new binary
	// inside the handler.
	writeTimeout    = 2 * time.Minute
	shutdownTimeout = 5 * time.Second
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
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

// serve runs the gateway until it fails or a successful self-update asks
// for a restart, in which case it re-execs the freshly replaced binary.
func serve(
	gateway *kobogateway.Server,
	cfg kobogateway.Config,
	stdout io.Writer,
) error {
	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      gateway.Handler(),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}

	fmt.Fprintf(
		stdout,
		"kobo-gateway %s (protocol v%d) listening on http://%s\n",
		Release,
		kobogateway.GatewayVersion,
		addr,
	)
	fmt.Fprintln(
		stdout,
		"leave this running, then return to the books page in your browser",
	)

	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()

	select {
	case err := <-errCh:
		return err
	case <-gateway.Restart():
		ctx, cancel := context.WithTimeout(
			context.Background(),
			shutdownTimeout,
		)
		defer cancel()
		_ = server.Shutdown(ctx)

		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("could not resolve executable to restart: %w", err)
		}

		fmt.Fprintln(stdout, "updated, restarting into the new binary…")

		//nolint:gosec //re-execs our own path as reported by os.Executable
		return syscall.Exec(executable, os.Args, os.Environ())
	}
}
