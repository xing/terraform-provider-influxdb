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
  name              = "custom-org-bucket"
  org               = var.influxdb_org
  description       = "Bucket with explicit organization"
  retention_seconds = 604800 # 7 days retention (7 * 24 * 60 * 60 seconds)
}

# Example bucket with 30-day retention
resource "influxdb_bucket" "short_retention" {
  name              = "short-retention-bucket"
  description       = "Bucket with 30-day retention policy"
  retention_seconds = 2592000 # 30 days (30 * 24 * 60 * 60 seconds)
}

# Example task with cron-based scheduling
resource "influxdb_task" "example_cron" {
  name        = "terraform-cron-task"
  description = "A task created by Terraform with cron scheduling"
  flux        = <<-EOT
    from(bucket: "short-retention-bucket")
      |> range(start: -24h)
      |> filter(fn: (r) => r._measurement == "temperature")
      |> aggregateWindow(every: 1h, fn: mean)
      |> to(bucket: "terraform-example-bucket")
  EOT
  cron        = "0 */6 * * *" # Every 6 hours
  status      = "active"
}

# Simple task with import (requires option task declaration)
resource "influxdb_task" "import_example" {
  name        = "import-example"
  description = "Simple task demonstrating import usage"
  flux        = <<-EOT
    import "array"
    
    option task = { name: "import-example", every: 5m }
    
    array.from(rows: [
      {_time: now(), _measurement: "test", _field: "value", _value: 1}
    ]) 
      |> to(bucket: "terraform-example-bucket")
  EOT
  every       = "5m"
  status      = "active"
}


# Example critical check with different threshold
resource "influxdb_check" "critical_memory_usage" {
  name                    = "terraform-memory-critical"
  description             = "Critical memory usage alert"
  query                   = <<-EOT
    from(bucket: "terraform-example-bucket")
      |> range(start: -1m)
      |> filter(fn: (r) => r._measurement == "memory")
      |> filter(fn: (r) => r._field == "used_percent")
      |> aggregateWindow(every: 1m, fn: max, createEmpty: false)
  EOT
  every                   = "1m"
  offset                  = "0s"
  status_message_template = "Memory usage is critical: $${r._value}%"
  status                  = "active"
  type                    = "threshold"

  thresholds {
    type       = "greater"
    value      = 90
    level      = "CRIT"
    all_values = false
  }
}


# Microsoft Teams notification endpoint
resource "influxdb_notification_endpoint" "teams_webhook" {
  name   = "microsoft-teams-webhook"
  type   = "http"
  status = "active"
  headers = {
    "Content-Type" = "application/json"
  }
  content_template = "{\"text\": \"🔔 InfluxDB Alert: Check $${r._check_name} is $${r._level} - Value: $${r._value}\"}"
  auth_method= "none"
  method = "POST"
}

# Notification rule that monitors all checks and sends to Teams  
resource "influxdb_notification_rule" "teams_cpu_alert" {
  name        = "teams-cpu-alert"
  description = "Sends alerts to Microsoft Teams for all check statuses"
  endpoint_id = influxdb_notification_endpoint.teams_webhook.id
  status      = "active"
  type        = "http"
  every       = "1m"
  offset      = "0s"
  
  status_rules {
    current_level = "OK"
  }
  
  status_rules {
    current_level = "INFO"  
  }
  
  status_rules {
    current_level = "WARN"
  }
  
  status_rules {
    current_level = "CRIT"
  }
}