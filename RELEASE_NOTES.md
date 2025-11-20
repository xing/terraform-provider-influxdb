# Release Notes

## v0.1.6 - 2025-11-20

### Features
- Initial stable release for Terraform Registry
- Support for InfluxDB bucket management
- Multi-platform binary support (darwin/linux/windows on amd64/arm64)

### Bug Fixes
- Fixed binary naming convention for Terraform Registry compatibility
- Corrected archive structure to meet registry requirements
- Removed unnecessary signature files that could cause validation issues

### Changes
- Optimized release process for Terraform Registry submission
- Updated Makefile to generate proper binary and archive naming
- Added comprehensive platform support

## v0.1.0 - 2025-11-20

## Features

- **InfluxDB Bucket Resource**: Create, read, update, and delete InfluxDB buckets
- **InfluxDB Task Resource**: Manage InfluxDB tasks with Flux queries
- **InfluxDB Check Resource**: Create and manage InfluxDB monitoring checks

## Provider Configuration

The provider supports the following configuration options:
- `url`: InfluxDB server URL
- `token`: Authentication token
- `org`: Default organization
- `bucket`: Default bucket (optional)

## Resources

### `influxdb_bucket`
- `name`: Bucket name (required)
- `org`: Organization (optional, uses provider default)
- `description`: Bucket description (optional)
- `retention_seconds`: Data retention period in seconds (optional, defaults to 0 for infinite retention)

### `influxdb_task`
- `name`: Task name (required)
- `org`: Organization (optional)
- `description`: Task description (optional)
- `flux`: Flux query script (required)
- `every`: Schedule interval (optional)
- `cron`: Cron expression for scheduling (optional)

### `influxdb_check`
- `name`: Check name (required)
- `org`: Organization (optional)
- `query`: Query configuration (required)
- `status`: Check status (optional)
- `type`: Check type (required)

## Installation

Download the appropriate binary for your platform and place it in your Terraform plugins directory, or use the GitHub source in your Terraform configuration:

```hcl
terraform {
  required_providers {
    influxdb = {
      source  = "github.com/xing/terraform-provider-influxdb"
      version = "~> 0.1"
    }
  }
}
```