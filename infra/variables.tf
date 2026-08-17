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

# Kamal (issue #1033) — see infra/README.md's "Deploy the app via Kamal"
# section. Only the GHCR username is a tfvar (needed inside the rendered
# config/deploy.yml itself); every other Kamal/app secret (KAMAL_REGISTRY_
# PASSWORD, the full do-app.yaml SECRET list) is exported by hand in the
# shell running `tofu apply`, same convention as HCLOUD_TOKEN — not
# duplicated here as tfvars.
variable "kamal_registry_username" {
  description = "GHCR username Kamal authenticates as to pull ghcr.io/xdoubleu/tools.xdoubleu.com/app."
  type        = string
}
