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

# Shared Docker network (issue #1033) between Postgres/GoTrue and the
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
      API_EXTERNAL_URL=${var.gotrue_site_url}
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

# Current git commit — deployed as RELEASE (issue #1033), and used as a
# redeploy trigger below so `tofu apply` after a new commit (whose image
# docker.yml's CI already pushed to GHCR) redeploys automatically, while
# re-applying with no new commit is a no-op.
data "external" "git_sha" {
  program = ["sh", "-c", "printf '{\"sha\":\"%s\"}' \"$(git -C ${path.module} rev-parse HEAD)\""]
}

# Renders config/deploy.yml (repo root, gitignored) from the committed
# template — see infra/templates/deploy.yml.tftpl's own header comment.
resource "local_file" "kamal_deploy_config" {
  filename = "${path.module}/../config/deploy.yml"
  content = templatefile("${path.module}/templates/deploy.yml.tftpl", {
    server_ip     = var.server_ip
    ghcr_username = var.kamal_registry_username
  })
}

# Drives the actual app deploy (issue #1033) — `tofu apply` is the single
# entrypoint for the whole stack, so this shells out to `kamal setup`
# locally rather than requiring a separate manual `kamal deploy` step.
# `gem install --conservative` (no-op if a satisfying version is already
# installed) means the operator doesn't need to install the Kamal gem by
# hand either — the only prerequisite left on the machine running
# `tofu apply` is Ruby 3.0+ itself, which Tofu can't install for you.
# `kamal setup` is idempotent/safe to re-run (installs kamal-proxy only if
# missing, otherwise just deploys), so it's used uniformly instead of
# branching between `setup` (first run) and `deploy` (later runs).
#
# DB_DSN/GOTRUE_URL/RELEASE are the only Kamal secrets Tofu injects directly
# (values it uniquely knows or computes) — every other secret
# config/deploy.yml references (the do-app.yaml SECRET list,
# KAMAL_REGISTRY_PASSWORD) is exported by hand in the shell running
# `tofu apply`, same convention as HCLOUD_TOKEN; local-exec inherits the
# parent process's environment, so `.kamal/secrets`' `VAR=$VAR` lines
# resolve those without Tofu needing to duplicate them as tfvars.
#
# Kamal's own build-vs-pull behavior for an externally CI-pushed image
# (config/deploy.yml has no explicit image tag) isn't fully pinned down
# here — worth confirming during the first real deploy; #1036 (automate
# Kamal deploys in CI) is the natural place to nail that down for good.
resource "null_resource" "kamal_deploy" {
  depends_on = [null_resource.postgres, local_file.kamal_deploy_config]

  triggers = {
    deploy_config_hash = local_file.kamal_deploy_config.content_sha256
    git_sha            = data.external.git_sha.result.sha
  }

  provisioner "local-exec" {
    working_dir = "${path.module}/.."
    command     = "gem install kamal --no-document --conservative && kamal setup"
    environment = {
      RELEASE    = data.external.git_sha.result.sha
      DB_DSN     = "postgres://postgres:${random_password.postgres.result}@postgres:5432/postgres"
      GOTRUE_URL = "http://gotrue:9999"
    }
  }
}
