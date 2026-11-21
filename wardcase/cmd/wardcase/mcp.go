package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"tools.xdoubleu.com/wardcase/internal/ward"
)

const mcpUsage = "usage: wardcase mcp login <name> | list | logout <name>"

// openBrowser shows url with macOS's `open`.
func openBrowser(url string) error {
	cmd := exec.CommandContext(context.Background(), "open", url)
	return cmd.Run()
}

// mcp runs `wardcase mcp login|list|logout`.
func mcp(
	ctx context.Context, h host, home string, args []string, out, errOut io.Writer,
) int {
	cfg, err := ward.LoadConfig(ward.ConfigPath(home))
	if err != nil {
		fmt.Fprintln(errOut, "wardcase:", err)
		return 1
	}
	verb, name, _ := strings.Cut(strings.Join(args, " "), " ")
	if verb == "list" && name == "" {
		return mcpList(cfg, h.secrets, out, errOut)
	}
	if name == "" || (verb != "login" && verb != "logout") {
		fmt.Fprintln(errOut, mcpUsage)
		return exitUsage
	}
	if _, ok := cfg.MCPServer(name); !ok {
		fmt.Fprintf(errOut, "wardcase: no [[mcp]] named %q in %s\n", name,
			ward.ConfigPath(home))
		return 1
	}
	if verb == "logout" {
		err = ward.LogoutMCP(h.secrets, name)
	} else {
		err = ward.LoginMCP(ctx, cfg, h.secrets,
			ward.MCPLogin{HTTP: nil, Open: openBrowser, Out: out}, name)
	}
	if err != nil {
		fmt.Fprintln(errOut, "wardcase:", err)
		return 1
	}
	fmt.Fprintf(out, "%s: done (%s).\n", name, verb)
	return 0
}

func mcpList(cfg ward.Config, store ward.Secrets, out, errOut io.Writer) int {
	for _, m := range cfg.MCP {
		signedIn, err := ward.MCPSignedIn(store, m.Name)
		if err != nil {
			fmt.Fprintln(errOut, "wardcase:", err)
			return 1
		}
		state := "not signed in"
		if signedIn {
			state = "signed in"
		}
		fmt.Fprintf(out, "%s\t%s\t%s\tjobs=%t\n", m.Name, m.URL, state, m.Jobs)
	}
	return 0
}
