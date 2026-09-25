# Non-literal entries are .pub paths (tfvars can't interpolate `$(cat ...)`).
locals {
  deploy_ssh_public_keys = [
    for key in var.deploy_ssh_public_keys :
    can(regex("^(ssh-|ecdsa-)", key)) ? key : trimspace(file(pathexpand(key)))
  ]
}

# Attached by ID to the manually created server (infra/README.md).
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

# Runs the idempotent harden.sh as `deploy` via sudo; re-runs when it changes.
# Never root: harden.sh sets PermitRootLogin no, so root SSH only works on a
# brand-new server's first run, which is a manual step (infra/README.md).
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

# Authorizes key changes as `deploy`, never root, so it can re-run any time.
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
      # POSIX sh only: remote-exec uses dash, which rejects `<<<`.
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

# Kept in local Tofu state; read with `tofu output -raw postgres_password`.
resource "random_password" "postgres" {
  length  = 32
  special = false # avoid shell-quoting issues in .env / remote-exec
}

output "postgres_password" {
  value     = random_password.postgres.result
  sensitive = true
}

# Postgres needs this external network to start, and Kamal needs Postgres
# for its health check, so it's created here rather than by `kamal setup`.
resource "null_resource" "kamal_network" {
  depends_on = [null_resource.harden]

  connection {
    type  = "ssh"
    host  = var.server_ip
    user  = "deploy"
    agent = true
  }

  provisioner "remote-exec" {
    # `kamal setup` also creates it if missing, so order doesn't matter.
    inline = ["docker network create kamal || true"]
  }
}

# Uploads the script/env for the timer harden.sh enables, kept here so edits
# don't change harden.sh's trigger hash.
resource "null_resource" "release_upgrade_check" {
  depends_on = [null_resource.harden]

  triggers = {
    script_hash = filesha256("${path.module}/release-upgrade-check.sh")
    env_hash = sha256(join("\n", [
      var.release_check_resend_api_key, var.release_check_email_from, var.release_check_email_to
    ]))
  }

  connection {
    type  = "ssh"
    host  = var.server_ip
    user  = "deploy"
    agent = true
  }

  provisioner "file" {
    source      = "${path.module}/release-upgrade-check.sh"
    destination = "/tmp/release-upgrade-check.sh"
  }

  provisioner "file" {
    content     = <<-EOT
      RESEND_API_KEY=${var.release_check_resend_api_key}
      NOTIFY_EMAIL_FROM=${var.release_check_email_from}
      NOTIFY_EMAIL_TO=${var.release_check_email_to}
    EOT
    destination = "/tmp/release-upgrade-check.env"
  }

  provisioner "remote-exec" {
    inline = [
      "sudo mv /tmp/release-upgrade-check.sh /usr/local/bin/release-upgrade-check.sh",
      "sudo chmod +x /usr/local/bin/release-upgrade-check.sh",
      "sudo mv /tmp/release-upgrade-check.env /etc/release-upgrade-check.env",
      "sudo chown root:root /etc/release-upgrade-check.env",
      "sudo chmod 600 /etc/release-upgrade-check.env",
      "sudo systemctl restart release-upgrade-check.timer",
    ]
  }
}

# Runs as `deploy` (in the docker group via harden.sh).
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

# Host metrics exporter; no secrets, so no .env file.
resource "null_resource" "node_exporter" {
  depends_on = [null_resource.harden, null_resource.kamal_network]

  triggers = {
    compose_hash = filesha256("${path.module}/node-exporter-compose.yml")
  }

  connection {
    type  = "ssh"
    host  = var.server_ip
    user  = "deploy"
    agent = true
  }

  provisioner "remote-exec" {
    inline = ["mkdir -p /home/deploy/node-exporter"]
  }

  provisioner "file" {
    source      = "${path.module}/node-exporter-compose.yml"
    destination = "/home/deploy/node-exporter/docker-compose.yml"
  }

  provisioner "remote-exec" {
    inline = [
      "cd /home/deploy/node-exporter && docker compose up -d --remove-orphans",
    ]
  }
}

# Prometheus + postgres_exporter; reuses random_password.postgres for
# postgres_exporter's DATA_SOURCE_NAME.
resource "null_resource" "prometheus" {
  depends_on = [null_resource.harden, null_resource.kamal_network, null_resource.postgres]

  triggers = {
    compose_hash = filesha256("${path.module}/prometheus-compose.yml")
    config_hash  = filesha256("${path.module}/prometheus.yml")
    # Hash, not the password itself.
    password_hash = sha256(random_password.postgres.result)
    # Restarts Prometheus when the /metrics bearer token rotates.
    ingest_secret_hash = sha256(var.observability_ingest_secret)
    # Setup commands live in a file so edits change a trigger; an inline
    # remote-exec edit changes no trigger and never runs.
    setup_hash = filesha256("${path.module}/prometheus-setup.sh")
  }

  connection {
    type  = "ssh"
    host  = var.server_ip
    user  = "deploy"
    agent = true
  }

  provisioner "remote-exec" {
    inline = ["mkdir -p /home/deploy/prometheus"]
  }

  provisioner "file" {
    source      = "${path.module}/prometheus-compose.yml"
    destination = "/home/deploy/prometheus/docker-compose.yml"
  }

  provisioner "file" {
    source      = "${path.module}/prometheus.yml"
    destination = "/home/deploy/prometheus/prometheus.yml"
  }

  provisioner "file" {
    content     = <<-EOT
      DATA_SOURCE_NAME=postgresql://postgres:${random_password.postgres.result}@postgres:5432/postgres?sslmode=disable
    EOT
    destination = "/home/deploy/prometheus/.env"
  }

  # Bearer token for the `web` scrape job; mounted read-only.
  provisioner "file" {
    content     = var.observability_ingest_secret
    destination = "/home/deploy/prometheus/web_ingest_secret"
  }

  provisioner "file" {
    source      = "${path.module}/prometheus-setup.sh"
    destination = "/home/deploy/prometheus/prometheus-setup.sh"
  }

  # See setup_hash in `triggers`.
  provisioner "remote-exec" {
    inline = ["cd /home/deploy/prometheus && bash prometheus-setup.sh"]
  }
}
