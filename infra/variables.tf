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

variable "deploy_ssh_public_keys" {
  description = "SSH public keys to authorize on the new non-root deploy user — your own key plus a dedicated, unencrypted CI deploy key (webfactory/ssh-agent runs headless and can't unlock a passphrase-protected key, so CI needs its own). Each entry is either the key's literal text (ssh-.../ecdsa-...) or a path to a .pub file (~ allowed), which Tofu reads for you."
  type        = list(string)

  # terraform.tfvars is not shell-interpolated, so an entry written as
  # "$(cat ~/.ssh/id.pub)" stays that literal string — harden.sh then appends
  # it to authorized_keys, where sshd silently ignores it and the key it was
  # meant to authorize simply never works (hit for real on the CI deploy key,
  # issue #1036). Paths are read by local.deploy_ssh_public_keys in main.tf;
  # a literal "$(cat ...)" is neither, so still fail at plan time.
  validation {
    condition     = alltrue([for key in var.deploy_ssh_public_keys : can(regex("^(ssh-|ecdsa-)", key)) || can(file(pathexpand(key)))])
    error_message = "Each entry must be the public key's literal text (ssh-/ecdsa-) or a readable path to a .pub file — not a $(cat ...) shell substitution, since .tfvars files are not shell-interpolated."
  }
}

# No app secrets here. Tofu provisions the host (firewall, hardening, deploy
# keys, Postgres) and nothing else — the app itself is deployed only
# by .github/workflows/main.yml's deploy-kamal job, which reads every app
# secret from repo Secrets (see infra/README.md's CI section). They used to
# be duplicated here to feed a local `kamal setup`, which meant rotating any
# one of them in two places.
