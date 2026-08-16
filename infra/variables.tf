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

variable "ssh_private_key_path" {
  description = "Path to the local SSH private key authorized on the server, used by the harden.sh provisioner to connect as root."
  type        = string
}

variable "deploy_ssh_public_key" {
  description = "SSH public key to authorize on the new non-root deploy user."
  type        = string
}
