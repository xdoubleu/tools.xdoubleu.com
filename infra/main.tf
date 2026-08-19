# Entries that aren't literal key text are paths to .pub files — read them
# here so terraform.tfvars can hold `~/.ssh/id.pub` instead of a key blob
# (.tfvars isn't shell-interpolated, so `$(cat ...)` can't do this).
locals {
  deploy_ssh_public_keys = [
    for key in var.deploy_ssh_public_keys :
    can(regex("^(ssh-|ecdsa-)", key)) ? key : trimspace(file(pathexpand(key)))
  ]
}

# Network firewall: a real Tofu-managed resource, attached to the manually
# created server by ID. See infra/README.md for the manual server-creation step.
resource "hcloud_firewall" "vps" {
  name = "tools-xdoubleu-com-vps"

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "80"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "443"
    source_ips = ["0.0.0.0/0", "::/0"]
  }
}

resource "hcloud_firewall_attachment" "vps" {
  firewall_id = hcloud_firewall.vps.id
  server_ids  = [var.server_id]
}

# OS-level hardening: cloud-init can't be used since the server is created
# manually (it only runs on first boot), so this connects as `deploy` (which
# harden.sh's own first run already created, with passwordless sudo) and
# runs the idempotent script via `sudo`. Re-runs automatically whenever
# harden.sh changes.
#
# NOT root, deliberately: harden.sh's own last step sets PermitRootLogin no,
# so root SSH only ever works on a server's very first-ever hardening run,
# before that line takes effect. Any resource that keeps trying to reconnect
# as root after that (a past version of this one did, twice — first via a
# keys_hash trigger, then simply because harden.sh's own content changed)
# fails auth forever and just hangs/retries. Since every server this repo
# manages has already been through that first bootstrap, deploy+sudo is
# always available going forward and root never needs to work again.
#
# Bootstrapping a genuinely new server from scratch (deploy doesn't exist
# yet) needs a one-time manual step first, outside Tofu: SSH in as root
# using the key set at server-creation time and run harden.sh by hand once
# — see infra/README.md's server-creation section.
resource "null_resource" "harden" {
  triggers = {
    script_hash = filesha256("${path.module}/harden.sh")
  }

  connection {
    type  = "ssh"
    host  = var.server_ip
    user  = "deploy"
    agent = true # picks up SSH_AUTH_SOCK; file(private_key) can't handle a passphrase-protected key
  }

  provisioner "file" {
    source      = "${path.module}/harden.sh"
    destination = "/tmp/harden.sh"
  }

  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/harden.sh",
      "sudo /tmp/harden.sh '${join("\n", local.deploy_ssh_public_keys)}'",
    ]
  }
}

# Authorizes new/changed keys after the initial bootstrap above — connects
# as `deploy` (already trusted once null_resource.harden has run once),
# never root, so this can safely re-run any time deploy_ssh_public_keys
# changes without hitting the permanent root-lockout problem described above.
resource "null_resource" "deploy_keys" {
  depends_on = [null_resource.harden]

  triggers = {
    keys_hash = sha256(join("\n", local.deploy_ssh_public_keys))
  }

  connection {
    type  = "ssh"
    host  = var.server_ip
    user  = "deploy"
    agent = true
  }

  provisioner "remote-exec" {
    inline = [
      # POSIX sh only: remote-exec runs this with /bin/sh (dash on Ubuntu),
      # so no herestrings (`<<<`) — dash exits 2 on those.
      <<-EOT
        AUTH_KEYS=/home/deploy/.ssh/authorized_keys
        echo '${join("\n", local.deploy_ssh_public_keys)}' | while IFS= read -r key; do
          [ -z "$key" ] && continue
          grep -qxF "$key" "$AUTH_KEYS" || echo "$key" >>"$AUTH_KEYS"
        done
      EOT
    ]
  }
}

# Postgres superuser password. Kept in local Tofu state (never committed,
# never used in CI, same trust boundary as the rest of infra/) rather than
# passed in externally — retrieve it with `tofu output -raw postgres_password`.
resource "random_password" "postgres" {
  length  = 32
  special = false # avoid shell-quoting issues in .env / remote-exec
}

output "postgres_password" {
  value     = random_password.postgres.result
  sensitive = true
}

# Shared Docker network (issue #1033) between Postgres and the
# Kamal-deployed app — postgres-compose.yml declares it `external: true`, so
# it must exist before `docker compose up` runs there. Created here (not left
# for `kamal setup` to create) to break that circular dependency: Postgres
# needs this network before it can start, but Kamal's own deploy needs
# Postgres reachable for its health check before it can run.
resource "null_resource" "kamal_network" {
  depends_on = [null_resource.harden]

  connection {
    type  = "ssh"
    host  = var.server_ip
    user  = "deploy"
    agent = true
  }

  provisioner "remote-exec" {
    # Idempotent — `kamal setup` also creates this network if it doesn't
    # find one, so re-running either side is safe regardless of order.
    inline = ["docker network create kamal || true"]
  }
}

# Stands up self-hosted Postgres (issue #1031) via Docker Compose, following
# the same file+remote-exec pattern as null_resource.harden above. Runs as
# `deploy`, not root, since harden.sh already put it in the docker group.
resource "null_resource" "postgres" {
  depends_on = [null_resource.harden, null_resource.kamal_network]

  triggers = {
    compose_hash  = filesha256("${path.module}/postgres-compose.yml")
    password_hash = sha256(random_password.postgres.result)
  }

  connection {
    type  = "ssh"
    host  = var.server_ip
    user  = "deploy"
    agent = true
  }

  provisioner "remote-exec" {
    inline = ["mkdir -p /home/deploy/postgres"]
  }

  provisioner "file" {
    source      = "${path.module}/postgres-compose.yml"
    destination = "/home/deploy/postgres/docker-compose.yml"
  }

  provisioner "file" {
    content     = <<-EOT
      POSTGRES_PASSWORD=${random_password.postgres.result}
    EOT
    destination = "/home/deploy/postgres/.env"
  }

  provisioner "remote-exec" {
    inline = [
      "chmod 600 /home/deploy/postgres/.env",
      "cd /home/deploy/postgres && docker compose up -d --remove-orphans",
    ]
  }
}
