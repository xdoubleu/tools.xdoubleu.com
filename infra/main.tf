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
# manually (it only runs on first boot), so this SSHes in as root once to run
# an idempotent script. Re-runs automatically whenever harden.sh changes.
resource "null_resource" "harden" {
  triggers = {
    script_hash = filesha256("${path.module}/harden.sh")
  }

  connection {
    type  = "ssh"
    host  = var.server_ip
    user  = "root"
    agent = true # picks up SSH_AUTH_SOCK; file(private_key) can't handle a passphrase-protected key
  }

  provisioner "file" {
    source      = "${path.module}/harden.sh"
    destination = "/root/harden.sh"
  }

  provisioner "remote-exec" {
    inline = [
      "chmod +x /root/harden.sh",
      "/root/harden.sh '${var.deploy_ssh_public_key}'",
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

# Stands up self-hosted Postgres (issue #1031) via Docker Compose, following
# the same file+remote-exec pattern as null_resource.harden above. Runs as
# `deploy`, not root, since harden.sh already put it in the docker group.
resource "null_resource" "postgres" {
  depends_on = [null_resource.harden]

  triggers = {
    compose_hash    = filesha256("${path.module}/postgres-compose.yml")
    password_hash   = sha256(random_password.postgres.result)
    jwt_secret_hash = sha256(var.gotrue_jwt_secret)
    resend_key_hash = sha256(var.resend_api_key)
    site_url_hash   = sha256(var.gotrue_site_url)
    smtp_email_hash = sha256(var.gotrue_smtp_admin_email)
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
      GOTRUE_JWT_SECRET=${var.gotrue_jwt_secret}
      GOTRUE_DB_DATABASE_URL=postgres://postgres:${random_password.postgres.result}@postgres:5432/postgres?search_path=auth
      GOTRUE_SMTP_PASS=${var.resend_api_key}
      GOTRUE_SITE_URL=${var.gotrue_site_url}
      GOTRUE_SMTP_ADMIN_EMAIL=${var.gotrue_smtp_admin_email}
    EOT
    destination = "/home/deploy/postgres/.env"
  }

  provisioner "remote-exec" {
    inline = [
      "chmod 600 /home/deploy/postgres/.env",
      "cd /home/deploy/postgres && docker compose up -d",
    ]
  }
}
