# How to Test Your InfluxDB Provider

This directory contains example Terraform configuration for testing your local InfluxDB provider.

## Prerequisites

### Start InfluxDB with Docker

First, start a local InfluxDB instance using Docker:

```bash
# Start InfluxDB container
docker run -d \
  --name influxdb \
  -p 8086:8086 \
  -e DOCKER_INFLUXDB_INIT_MODE=setup \
  -e DOCKER_INFLUXDB_INIT_USERNAME=admin \
  -e DOCKER_INFLUXDB_INIT_PASSWORD=password123 \
  -e DOCKER_INFLUXDB_INIT_ORG=my-org \
  -e DOCKER_INFLUXDB_INIT_BUCKET=my-bucket \
  -e DOCKER_INFLUXDB_INIT_ADMIN_TOKEN=my-super-secret-auth-token \
  influxdb:2.7

# Verify it's running
docker logs influxdb

# Access InfluxDB UI at http://localhost:8086
# Username: admin
# Password: password123
```

**Stop and cleanup:**
```bash
# Stop the container
docker stop influxdb

# Remove the container
docker rm influxdb
```

## Setup

1. **Build your provider first:**
   ```bash
   cd /Users/danylo.goncharov/Documents/Code/New-Work/InfluxDBProvider
   go build -o terraform-provider-influxdb
   ```

2. **Set up Terraform CLI configuration:**
   ```bash
   export TF_CLI_CONFIG_FILE=".terraformrc"
   ```

## Usage

```bash
terraform plan
terraform apply
```