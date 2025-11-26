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

# # Example task with duration-based scheduling
# resource "influxdb_task" "example_every" {
#   name        = "terraform-example-task"
#   description = "A task created by Terraform with duration scheduling"
#   flux        = <<-EOT
#     from(bucket: "terraform-example-bucket")
#       |> range(start: -1h)
#       |> filter(fn: (r) => r._measurement == "cpu")
#       |> mean()
#       |> to(bucket: "short-retention-bucket")
#   EOT
#   every       = "1h"
#   status      = "active"
# }

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

# Example check for monitoring bucket data
resource "influxdb_check" "high_cpu_usage" {
  name        = "terraform-high-cpu-check"
  description = "Monitor for high CPU usage in terraform-example-bucket"
  query       = <<-EOT
    from(bucket: "terraform-example-bucket")
      |> range(start: v.timeRangeStart, stop: v.timeRangeStop)
      |> filter(fn: (r) => r._measurement == "cpu")
      |> filter(fn: (r) => r._field == "usage_percent")
      |> aggregateWindow(every: 1m, fn: mean, createEmpty: false)
      |> yield(name: "mean")
  EOT
  every       = "1m"
  status      = "active"
  type        = "threshold"

  thresholds {
    type       = "greater"
    value      = 80.0
    level      = "WARN"
    all_values = false
  }
}

# Example critical check with different threshold
resource "influxdb_check" "critical_memory_usage" {
  name                    = "terraform-memory-critical"
  description             = "Critical memory usage alert"
  query                   = <<-EOT
    from(bucket: "terraform-example-bucket")
      |> range(start: v.timeRangeStart, stop: v.timeRangeStop)
      |> filter(fn: (r) => r._measurement == "memory")
      |> filter(fn: (r) => r._field == "used_percent")
      |> aggregateWindow(every: 5m, fn: max, createEmpty: false)
      |> yield(name: "max")
  EOT
  every                   = "5m"
  status_message_template = "Memory usage is critical: $${r._value}%"
  status                  = "active"
  type                    = "threshold"

  thresholds {
    type       = "greater"
    value      = 95.0
    level      = "CRIT"
    all_values = false
  }
}

# Example HTTP notification endpoint (no authentication required)
resource "influxdb_notification_endpoint" "webhook_endpoint" {
  name = "terraform-webhook-endpoint"
  type = "http"
  url  = "https://api.example.com/webhooks/influx"
  # token and auth_method are optional - omit them for endpoints that don't require auth
}

# Example notification rule for high CPU alerts to webhook
resource "influxdb_notification_rule" "cpu_alert_rule" {
  name             = "terraform-cpu-alert"
  description      = "Alert when CPU usage goes from OK to WARN or CRIT"
  endpoint_id      = influxdb_notification_endpoint.webhook_endpoint.id
  message_template = "🚨 CPU Alert: $${r._check_name} is $${r._level} - Usage: $${r._value}%"

  # Status rules - array of objects
  status_rules = [
    {
      current_level  = "WARN"
      previous_level = "OK"
    },
    {
      current_level = "CRIT"
    }
  ]

  # Tag rules - array of objects  
  tag_rules = [
    {
      key      = "host"
      value    = "production-web-*"
      operator = "equal"
    }
  ]
}

# Example notification rule for memory alerts to webhook
resource "influxdb_notification_rule" "memory_alert_rule" {
  name             = "terraform-memory-alert"
  description      = "Critical memory usage notifications"
  endpoint_id      = influxdb_notification_endpoint.webhook_endpoint.id
  message_template = "Memory Critical: $${r._check_name} on $${r.host} - $${r._value}% used"

  # Status rules - array of objects
  status_rules = [
    {
      current_level  = "CRIT"
      previous_level = "WARN"
    }
  ]

  # Tag rules - array of objects
  tag_rules = [
    {
      key      = "environment"
      value    = "production"
      operator = "equal"
    }
  ]
}