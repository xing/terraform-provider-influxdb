# Terraform Provider for InfluxDB

A Terraform provider for managing InfluxDB resources.

## Features

This provider supports managing the following InfluxDB resources:

- **Buckets** (`influxdb_bucket`) - Create and manage data storage buckets with retention policies
- **Tasks** (`influxdb_task`) - Create and manage scheduled Flux query tasks
- **Checks** (`influxdb_check`) - Create and manage monitoring checks

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.21 (for development)
- InfluxDB 2.x instance

## Using the Provider

### Installation

The provider will be available from the Terraform Registry. Add this to your Terraform configuration:

```hcl
terraform {
  required_providers {
    influxdb = {
      source = "xing/influxdb"
    }
  }
}
```

### Configuration

```hcl
provider "influxdb" {
  url   = "http://localhost:8086"
  token = "your-influxdb-token"
  org   = "your-organization"
}
```

### Example Usage

#### Creating a Bucket

```hcl
resource "influxdb_bucket" "example" {
  name              = "my-bucket"
  description       = "Example bucket"
  retention_seconds = 604800  # 7 days
}
```

#### Creating a Task

```hcl
resource "influxdb_task" "example" {
  name        = "my-task"
  description = "Example task"
  every       = "1h"
  flux        = <<-EOT
    from(bucket: "my-bucket")
      |> range(start: -1h)
      |> mean()
      |> to(bucket: "processed-data")
  EOT
}
```

### Local Development

See the [examples README](./examples/README.md) for detailed instructions on setting up and testing the provider locally.

## Documentation

Detailed documentation for each resource is available in the [docs](./docs) directory.

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -am 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.