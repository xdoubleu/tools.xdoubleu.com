package constants

import "tools.xdoubleu.com/internal/contexttools"

const UserContextKey = contexttools.Key("user")

const AppDisplayNameContextKey = contexttools.Key("appDisplayName")
const OriginalPathContextKey = contexttools.Key("originalPath")

// AuthChannelContextKey holds how the request was authenticated.
const AuthChannelContextKey = contexttools.Key("authChannel")

const (
	// ChannelBrowser is a cookie session from the web app.
	ChannelBrowser = "browser"
	// ChannelMCP is an OAuth bearer token on an MCP endpoint.
	ChannelMCP = "mcp"
)
