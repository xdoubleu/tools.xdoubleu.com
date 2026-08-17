variable "hcloud_token" {
  description = "Hetzner Cloud API token (read+write). Export as TF_VAR_hcloud_token or pass with -var."
  type        = string
  sensitive   = true
}

variable "server_id" {
  description = "ID of the Hetzner server, created manually via the console, that the firewall attaches to and harden.sh runs against."
  type        = string
}

variable "server_ip" {
  description = "Public IPv4 of the manually-created server, used for the hardening provisioner's SSH connection."
  type        = string
}

variable "deploy_ssh_public_key" {
  description = "SSH public key to authorize on the new non-root deploy user."
  type        = string
}

# GoTrue (issue #1032) — see infra/README.md's "Stand up GoTrue" section for
# where to find each of these.
variable "gotrue_jwt_secret" {
  description = "GOTRUE_JWT_SECRET, pulled from the Supabase dashboard (Project Settings -> API) so already-issued client JWTs keep validating post-cutover — must NOT be freshly generated."
  type        = string
  sensitive   = true
}

variable "resend_api_key" {
  description = "Resend API key, used as the password for GoTrue's SMTP relay (smtp.resend.com), same account as api/internal/mailer."
  type        = string
  sensitive   = true
}

variable "gotrue_site_url" {
  description = "GOTRUE_SITE_URL — the app's public URL, used in auth emails/redirects."
  type        = string
}

variable "gotrue_smtp_admin_email" {
  description = "GOTRUE_SMTP_ADMIN_EMAIL — the From address for GoTrue's own auth emails."
  type        = string
}

# App secrets for the Kamal deploy (issue #1033) — the do-app.yaml SECRET
# list, relocated here rather than DO App Platform's secret store, same
# move already made for gotrue_jwt_secret/resend_api_key above. See
# infra/README.md's "Deploy the app via Kamal" section for where to find
# each value (same sources as today's DO App Platform deploy).
# RESEND_API_KEY isn't repeated here — it reuses var.resend_api_key above
# (same Resend account/key, already required for GoTrue's SMTP password).
# No GHCR registry credential var: ghcr.io/xdoubleu/tools.xdoubleu.com/app
# is a public package (do-app.yaml pulls it with none configured either).
# SUPABASE_PROJ_REF/SUPABASE_URL/SUPABASE_ANON_KEY only ever mattered for
# two things: the now-bypassed default GoTrue client (WithCustomAuthURL
# overrides it, see api/cmd/api/main.go) and the MCP OAuth 2.1 authorization-
# server issuer (api/cmd/api/mcp.go), which #1032 deliberately left pointed
# at Supabase rather than fixing GoTrue's missing dynamic-client-registration
# support. If Supabase isn't in the picture at all anymore, dummy values are
# fine here — MCP sign-in just stays broken until a future issue (#1039,
# "retire GoTrue entirely") replaces that flow properly; nothing else in the
# app depends on these three.
variable "supabase_proj_ref" {
  type      = string
  sensitive = true
}

# Not required even with a real Supabase project: auth-go sends this as a
# plain `apiKey` header on every request, but that's a Supabase-Kong-gateway
# concept — bare self-hosted GoTrue (no Kong in front, see
# infra/postgres-compose.yml) never validates it.
variable "supabase_api_key" {
  type      = string
  sensitive = true
}

variable "steam_api_key" {
  type      = string
  sensitive = true
}

variable "hardcover_api_key" {
  type      = string
  sensitive = true
}

variable "r2_account_id" {
  type      = string
  sensitive = true
}

variable "r2_access_key_id" {
  type      = string
  sensitive = true
}

variable "r2_secret_access_key" {
  type      = string
  sensitive = true
}

variable "r2_bucket" {
  type      = string
  sensitive = true
}

variable "sentry_dsn" {
  type      = string
  sensitive = true
}

variable "sentry_dsn_web" {
  type      = string
  sensitive = true
}

variable "supabase_url" {
  type      = string
  sensitive = true
}

variable "supabase_anon_key" {
  type      = string
  sensitive = true
}

variable "github_oauth_client_id" {
  type      = string
  sensitive = true
}

variable "github_oauth_client_secret" {
  type      = string
  sensitive = true
}

variable "sentry_oauth_client_id" {
  type      = string
  sensitive = true
}

variable "sentry_oauth_client_secret" {
  type      = string
  sensitive = true
}

variable "do_oauth_client_id" {
  type      = string
  sensitive = true
}

variable "do_oauth_client_secret" {
  type      = string
  sensitive = true
}

variable "encryption_key" {
  description = "32 raw bytes, base64-standard-encoded (e.g. `openssl rand -base64 32`). Encrypts stored OAuth tokens at rest (global.oauth_connections) — must match the value already in use, not a freshly generated one, or existing encrypted rows become undecryptable."
  type        = string
  sensitive   = true
}

variable "email_from" {
  type      = string
  sensitive = true
}

variable "notify_email_to" {
  type      = string
  sensitive = true
}

variable "email_inbound_domain" {
  type      = string
  sensitive = true
}

variable "email_inbound_secret" {
  type      = string
  sensitive = true
}
