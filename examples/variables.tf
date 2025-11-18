variable "influxdb_url" {
  description = "InfluxDB URL"
  type        = string
  default     = "http://localhost:8086"
}

variable "influxdb_token" {
  description = "InfluxDB authentication token"
  type        = string
  sensitive   = true
}

variable "influxdb_org" {
  description = "InfluxDB organization"
  type        = string
  default     = "my-org"
}

variable "influxdb_bucket" {
  description = "Default InfluxDB bucket"
  type        = string
  default     = "my-bucket"
}