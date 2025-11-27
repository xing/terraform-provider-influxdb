#!/bin/bash

# Load variables from terraform.tfvars
source_vars() {
    if [ -f terraform.tfvars ]; then
        export INFLUX_URL=$(grep influxdb_url terraform.tfvars | cut -d'"' -f2)
        export INFLUX_TOKEN=$(grep influxdb_token terraform.tfvars | cut -d'"' -f2)
        export INFLUX_ORG=$(grep influxdb_org terraform.tfvars | cut -d'"' -f2)
        export INFLUX_BUCKET=$(grep influxdb_bucket terraform.tfvars | cut -d'"' -f2)
    else
        echo "terraform.tfvars not found. Please create it with your InfluxDB credentials."
        exit 1
    fi
}

# Generate fake CPU data to trigger alerts
generate_cpu_data() {
    echo "Generating CPU data..."
    
    # Generate normal CPU usage (should not trigger alert)
    for i in {1..5}; do
        timestamp=$(date -u +%s)000000000  # nanoseconds
        cpu_usage=$((RANDOM % 70 + 10))    # 10-80% CPU usage
        
        curl -X POST "${INFLUX_URL}/api/v2/write?org=${INFLUX_ORG}&bucket=terraform-example-bucket&precision=ns" \
          -H "Authorization: Token ${INFLUX_TOKEN}" \
          -H "Content-Type: text/plain; charset=utf-8" \
          -d "cpu,host=server01 usage=${cpu_usage} ${timestamp}"
        
        echo "Written CPU usage: ${cpu_usage}%"
        sleep 1
    done
    
    # Generate high CPU usage to trigger WARN alert (>80%)
    echo "Generating high CPU data to trigger WARN alert..."
    for i in {1..3}; do
        timestamp=$(date -u +%s)000000000
        cpu_usage=$((RANDOM % 15 + 85))    # 85-99% CPU usage
        
        curl -X POST "${INFLUX_URL}/api/v2/write?org=${INFLUX_ORG}&bucket=terraform-example-bucket&precision=ns" \
          -H "Authorization: Token ${INFLUX_TOKEN}" \
          -H "Content-Type: text/plain; charset=utf-8" \
          -d "cpu,host=server01 usage=${cpu_usage} ${timestamp}"
        
        echo "Written high CPU usage: ${cpu_usage}%"
        sleep 1
    done
}

# Generate fake memory data to trigger critical alerts
generate_memory_data() {
    echo "Generating memory data..."
    
    # Generate normal memory usage
    for i in {1..5}; do
        timestamp=$(date -u +%s)000000000
        memory_usage=$((RANDOM % 80 + 10))  # 10-90% memory usage
        
        curl -X POST "${INFLUX_URL}/api/v2/write?org=${INFLUX_ORG}&bucket=terraform-example-bucket&precision=ns" \
          -H "Authorization: Token ${INFLUX_TOKEN}" \
          -H "Content-Type: text/plain; charset=utf-8" \
          -d "memory,host=server01 used_percent=${memory_usage} ${timestamp}"
        
        echo "Written memory usage: ${memory_usage}%"
        sleep 1
    done
    
    # Generate critical memory usage to trigger CRIT alert (>95%)
    echo "Generating critical memory data to trigger CRIT alert..."
    for i in {1..3}; do
        timestamp=$(date -u +%s)000000000
        memory_usage=$((RANDOM % 4 + 96))   # 96-99% memory usage
        
        curl -X POST "${INFLUX_URL}/api/v2/write?org=${INFLUX_ORG}&bucket=terraform-example-bucket&precision=ns" \
          -H "Authorization: Token ${INFLUX_TOKEN}" \
          -H "Content-Type: text/plain; charset=utf-8" \
          -d "memory,host=server01 used_percent=${memory_usage} ${timestamp}"
        
        echo "Written critical memory usage: ${memory_usage}%"
        sleep 1
    done
}

# Generate recovery data (low values to trigger OK status)
generate_recovery_data() {
    echo "Generating recovery data..."
    
    # Generate low CPU usage to trigger OK status
    for i in {1..3}; do
        timestamp=$(date -u +%s)000000000
        cpu_usage=$((RANDOM % 30 + 10))     # 10-40% CPU usage
        
        curl -X POST "${INFLUX_URL}/api/v2/write?org=${INFLUX_ORG}&bucket=terraform-example-bucket&precision=ns" \
          -H "Authorization: Token ${INFLUX_TOKEN}" \
          -H "Content-Type: text/plain; charset=utf-8" \
          -d "cpu,host=server01 usage=${cpu_usage} ${timestamp}"
        
        echo "Written recovery CPU usage: ${cpu_usage}%"
        sleep 1
    done
    
    # Generate low memory usage to trigger OK status
    for i in {1..3}; do
        timestamp=$(date -u +%s)000000000
        memory_usage=$((RANDOM % 40 + 30))  # 30-70% memory usage
        
        curl -X POST "${INFLUX_URL}/api/v2/write?org=${INFLUX_ORG}&bucket=terraform-example-bucket&precision=ns" \
          -H "Authorization: Token ${INFLUX_TOKEN}" \
          -H "Content-Type: text/plain; charset=utf-8" \
          -d "memory,host=server01 used_percent=${memory_usage} ${timestamp}"
        
        echo "Written recovery memory usage: ${memory_usage}%"
        sleep 1
    done
}

# Main execution
source_vars

case "${1:-all}" in
    "cpu")
        generate_cpu_data
        ;;
    "memory")
        generate_memory_data
        ;;
    "recovery")
        generate_recovery_data
        ;;
    "all")
        generate_cpu_data
        echo "Waiting 10 seconds before memory data..."
        sleep 10
        generate_memory_data
        echo "Waiting 10 seconds before recovery data..."
        sleep 10
        generate_recovery_data
        ;;
    *)
        echo "Usage: $0 [cpu|memory|recovery|all]"
        echo "  cpu      - Generate CPU data to trigger WARN alerts"
        echo "  memory   - Generate memory data to trigger CRIT alerts"
        echo "  recovery - Generate low values to trigger OK status"
        echo "  all      - Generate all data types in sequence (default)"
        exit 1
        ;;
esac

echo "Done! Check your Microsoft Teams channel for notifications."
echo "You can also check InfluxDB UI for the data and check statuses."