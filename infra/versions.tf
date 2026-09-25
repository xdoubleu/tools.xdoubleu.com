# State lives in Cloudflare R2. `endpoints.s3` (embeds the account ID) comes
# from `-backend-config` at init, since a backend block can't use variables.
terraform {
  required_version = ">= 1.10.0" # s3 backend's use_lockfile needs 1.10+

  backend "s3" {
    bucket = "tools-xdoubleu-com-tfstate"
    key    = "infra/terraform.tfstate"
    region = "auto" # R2 has no real regions; the s3 backend still requires a value

    use_lockfile = true # R2 has no DynamoDB equivalent; this is native S3-backend locking (OpenTofu 1.10+)

    # R2's S3-compatible API doesn't implement everything AWS S3 does.
    skip_credentials_validation = true
    skip_region_validation      = true
    skip_requesting_account_id  = true
    skip_s3_checksum            = true
  }

  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "~> 1.48"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }
}

provider "hcloud" {
  token = var.hcloud_token
}
