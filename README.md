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

## Development

### Building and Testing

```bash
# Build for current platform
make build

# Run tests
make test

# Install locally for development
make install

# Format and lint code
make fmt
make lint
```

### Releasing

This project uses [GoReleaser](https://goreleaser.com/) for releases. See [RELEASE.md](./RELEASE.md) for detailed release instructions.

**Quick release:**
```bash
make release-notes VERSION=v0.1.8    # Edit RELEASE_NOTES.md
git add RELEASE_NOTES.md && git commit -m "Release v0.1.8" && git tag v0.1.8
make goreleaser-release VERSION=v0.1.8
```

## Documentation

Detailed documentation for each resource is available in the [docs](./docs) directory.

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes and add tests
4. Run `make test fmt lint` to validate changes
5. Commit your changes (`git commit -am 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

Follow [Terraform provider best practices](https://developer.hashicorp.com/terraform/plugin/best-practices) for development.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.