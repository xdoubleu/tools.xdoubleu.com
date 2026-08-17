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
# Every secret config/deploy.yml references is a sensitive tfvar
# (infra/variables.tf), same convention as gotrue_jwt_secret/resend_api_key
# above — set once in terraform.tfvars, not re-exported by hand each time.
# DB_DSN/GOTRUE_URL/RELEASE are the only three Tofu computes itself rather
# than taking as a var (values it uniquely knows: the Postgres password it
# generated, the fixed in-network GoTrue URL, and the current git commit).
#
# This coexists with the deploy-kamal CI job (.github/workflows/main.yml,
# issue #1036) — that job handles routine push-to-main deploys once this
# resource has bootstrapped the host at least once (kamal-proxy installed,
# accessories-free setup done); this resource stays useful for one-off
# local deploys, secret rotations, and the rollback test below.
resource "null_resource" "kamal_deploy" {
  depends_on = [null_resource.postgres, local_file.kamal_deploy_config]

  triggers = {
    deploy_config_hash = local_file.kamal_deploy_config.content_sha256
    git_sha            = data.external.git_sha.result.sha
    secrets_hash = sha256(join("", [
      var.kamal_registry_password,
      var.supabase_proj_ref, var.supabase_api_key, var.steam_api_key,
      var.hardcover_api_key, var.r2_account_id, var.r2_access_key_id,
      var.r2_secret_access_key, var.r2_bucket, var.sentry_dsn,
      var.sentry_dsn_web, var.supabase_url, var.supabase_anon_key,
      var.github_oauth_client_id, var.github_oauth_client_secret,
      var.sentry_oauth_client_id, var.sentry_oauth_client_secret,
      var.do_oauth_client_id, var.do_oauth_client_secret,
      var.encryption_key, var.email_from, var.notify_email_to,
      var.email_inbound_domain, var.email_inbound_secret,
    ]))
  }

  provisioner "local-exec" {
    working_dir = "${path.module}/.."
    # --skip-push: the image is already built and pushed to GHCR by
    # docker.yml's CI job — kamal setup must not try to build it locally
    # (needs Docker + amd64 cross-build, and would deploy a locally-built
    # image instead of what CI actually tested). --version pins the exact
    # tag to pull: docker.yml's docker/metadata-action step tags images
    # `type=sha,prefix=`, which defaults to a 7-character short SHA — not
    # the full 40-character git_sha this resource otherwise uses for
    # RELEASE, confirmed via `kamal config`'s `absolute_image` output.
    command = "gem install kamal --no-document --conservative && kamal setup --skip-push --version=${substr(data.external.git_sha.result.sha, 0, 7)}"
    environment = {
      RELEASE    = data.external.git_sha.result.sha
      DB_DSN     = "postgres://postgres:${random_password.postgres.result}@postgres:5432/postgres"
      GOTRUE_URL = "http://gotrue:9999"

      KAMAL_REGISTRY_PASSWORD = var.kamal_registry_password

      SUPABASE_PROJ_REF          = var.supabase_proj_ref
      SUPABASE_API_KEY           = var.supabase_api_key
      STEAM_API_KEY              = var.steam_api_key
      HARDCOVER_API_KEY          = var.hardcover_api_key
      R2_ACCOUNT_ID              = var.r2_account_id
      R2_ACCESS_KEY_ID           = var.r2_access_key_id
      R2_SECRET_ACCESS_KEY       = var.r2_secret_access_key
      R2_BUCKET                  = var.r2_bucket
      SENTRY_DSN                 = var.sentry_dsn
      SENTRY_DSN_WEB             = var.sentry_dsn_web
      SUPABASE_URL               = var.supabase_url
      SUPABASE_ANON_KEY          = var.supabase_anon_key
      GITHUB_OAUTH_CLIENT_ID     = var.github_oauth_client_id
      GITHUB_OAUTH_CLIENT_SECRET = var.github_oauth_client_secret
      SENTRY_OAUTH_CLIENT_ID     = var.sentry_oauth_client_id
      SENTRY_OAUTH_CLIENT_SECRET = var.sentry_oauth_client_secret
      DO_OAUTH_CLIENT_ID         = var.do_oauth_client_id
      DO_OAUTH_CLIENT_SECRET     = var.do_oauth_client_secret
      ENCRYPTION_KEY             = var.encryption_key
      RESEND_API_KEY             = var.resend_api_key
      EMAIL_FROM                 = var.email_from
      NOTIFY_EMAIL_TO            = var.notify_email_to
      EMAIL_INBOUND_DOMAIN       = var.email_inbound_domain
      EMAIL_INBOUND_SECRET       = var.email_inbound_secret
    }
  }
}
