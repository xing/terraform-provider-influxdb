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

# Example bucket resource
resource "influxdb_bucket" "example" {
  name        = "terraform-example-bucket"
  description = "A bucket created by Terraform"
  # org is optional - will use provider default if not specified
}

# Example bucket with explicit org
resource "influxdb_bucket" "custom_org" {
  name        = "custom-org-bucket"
  org         = var.influxdb_org
  description = "Bucket with explicit organization"
}