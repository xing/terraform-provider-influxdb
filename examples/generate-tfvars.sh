#!/bin/bash
# Generate terraform.tfvars from .env file

set -a
source .env
set +a

cat > terraform.tfvars << EOF
# Auto-generated from .env file - do not edit manually
# Regenerate with: ./generate-tfvars.sh

influxdb_url    = "$INFLUXDB_URL"
influxdb_token  = "$INFLUXDB_TOKEN"
influxdb_org    = "$INFLUXDB_ORG"
influxdb_bucket = "$INFLUXDB_BUCKET"
EOF

echo "Generated terraform.tfvars from .env file"