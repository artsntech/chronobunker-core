#!/usr/bin/env bash
# Validates that required local services are reachable before starting the platform.
# Exits with a clear error message if any service is down.

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

check_port() {
    local host="$1"
    local port="$2"
    local service_name="$3"

    if command -v nc >/dev/null 2>&1; then
        if nc -z -w 2 "$host" "$port" 2>/dev/null; then
            echo -e "${GREEN}[OK]${NC}   $service_name is reachable at $host:$port"
            return 0
        fi
    elif command -v timeout >/dev/null 2>&1; then
        if timeout 2 bash -c "</dev/tcp/$host/$port" 2>/dev/null; then
            echo -e "${GREEN}[OK]${NC}   $service_name is reachable at $host:$port"
            return 0
        fi
    else
        echo -e "${YELLOW}[WARN]${NC} Neither 'nc' nor 'timeout' is available. Cannot validate $service_name."
        return 0
    fi

    echo -e "${RED}[FAIL]${NC} $service_name is NOT reachable at $host:$port"
    return 1
}

echo "Validating local infrastructure..."

FAILED=0

check_port "localhost" "5432" "PostgreSQL" || FAILED=1
check_port "localhost" "6379" "Redis"      || FAILED=1

if [ "${CHRONO_KAFKA_ENABLED:-false}" = "true" ]; then
    check_port "localhost" "9092" "Kafka" || FAILED=1
fi

if [ "$FAILED" -ne 0 ]; then
    echo ""
    echo -e "${RED}Infrastructure validation failed.${NC}"
    echo "Please start the missing services before running 'make dev-api' or 'make dev-worker'."
    echo "On Windows, use scripts/check_ports.bat instead."
    exit 1
fi

echo ""
echo -e "${GREEN}All required services are reachable.${NC}"