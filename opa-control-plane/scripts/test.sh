#!/bin/bash
set -e

echo "Running OPA Database Integration tests..."

# Wait for services
sleep 10

# Test bundle server
echo "Testing bundle server..."
curl -f http://localhost:8080/health || exit 1
curl -f http://localhost:8080/status || exit 1

# Test OPA
echo "Testing OPA server..."
curl -f http://localhost:8181/health || exit 1

# Test policy queries
echo "Testing RBAC policy..."
curl -X POST http://localhost:8181/v1/data/rbac/allow \
  -H "Content-Type: application/json" \
  -d '{"input": {"user": {"roles": ["admin"]}, "required_role": "user"}}' || exit 1

echo "All tests passed!"