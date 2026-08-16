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
