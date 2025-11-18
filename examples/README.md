# How to Test Your InfluxDB Provider

This directory contains example Terraform configuration for testing your local InfluxDB provider.

## Prerequisites

### Start InfluxDB with Docker

All configuration is centralized in the `.env` file. Start InfluxDB using the helper script:

```bash
# Start InfluxDB with configuration from .env file
./start-influxdb.sh

# Verify it's running
docker logs influxdb

# Access InfluxDB UI (URL from .env file)
echo "Access InfluxDB UI at: $INFLUXDB_URL"
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

3. **Generate terraform.tfvars from .env:**
   ```bash
   ./generate-tfvars.sh
   ```

## Usage

```bash
./generate-tfvars.sh

# Skip terraform init when using dev_overrides
# Plan the configuration (uses your local provider)
terraform plan

terraform apply
```