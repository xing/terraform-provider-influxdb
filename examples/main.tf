terraform {
  required_version = ">= 1.0"
  
  required_providers {
    influxdb = {
      source  = "registry.terraform.io/new-work/influxdb"
      version = "~> 1.0"
    }
  }
}

provider "influxdb" {
  url    = var.influxdb_url
  token  = var.influxdb_token
  org    = var.influxdb_org
  bucket = var.influxdb_bucket
}

# Example bucket resource with infinite retention
resource "influxdb_bucket" "example" {
  name        = "terraform-example-bucket"
  description = "A bucket created by Terraform"
  # org is optional - will use provider default if not specified
  # retention_seconds is optional - defaults to 0 (infinite retention)
}

# Example bucket with explicit org and retention policy
resource "influxdb_bucket" "custom_org" {
  name             = "custom-org-bucket"
  org              = var.influxdb_org
  description      = "Bucket with explicit organization"
  retention_seconds = 604800  # 7 days retention (7 * 24 * 60 * 60 seconds)
}

# Example bucket with 30-day retention
resource "influxdb_bucket" "short_retention" {
  name             = "short-retention-bucket"
  description      = "Bucket with 30-day retention policy"
  retention_seconds = 2592000  # 30 days (30 * 24 * 60 * 60 seconds)
}