#!/bin/bash
# Source environment variables for InfluxDB
set -a
source .env
set +a

# Start InfluxDB container with environment variables
docker run -d \
  --name influxdb \
  -p 8086:8086 \
  -e DOCKER_INFLUXDB_INIT_MODE=setup \
  -e DOCKER_INFLUXDB_INIT_USERNAME="$INFLUXDB_ADMIN_USERNAME" \
  -e DOCKER_INFLUXDB_INIT_PASSWORD="$INFLUXDB_ADMIN_PASSWORD" \
  -e DOCKER_INFLUXDB_INIT_ORG="$INFLUXDB_ORG" \
  -e DOCKER_INFLUXDB_INIT_BUCKET="$INFLUXDB_BUCKET" \
  -e DOCKER_INFLUXDB_INIT_ADMIN_TOKEN="$INFLUXDB_TOKEN" \
  influxdb:2.7

echo "InfluxDB started with configuration from .env file"
echo "Access InfluxDB UI at: $INFLUXDB_URL"
echo "Username: $INFLUXDB_ADMIN_USERNAME"
echo "Password: $INFLUXDB_ADMIN_PASSWORD"