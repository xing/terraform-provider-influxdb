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

# Example configuration - you can add resources here when you implement them
# resource "influxdb_bucket" "example" {
#   name = "my-bucket"
#   org  = var.influxdb_org
# }

# data "influxdb_buckets" "all" {
#   org = var.influxdb_org
# }