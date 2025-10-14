#!/bin/bash

echo "Cleaning up OPA Database Integration..."

# Stop services
docker-compose down -v --remove-orphans

# Remove generated files
rm -rf logs/
rm -rf tmp/

# Clean Docker
docker system prune -f

echo "Cleanup completed!"