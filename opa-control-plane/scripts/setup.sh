#!/bin/bash
set -e

echo "Setting up OPA Database Integration project..."

# Create .env if it doesn't exist
if [ ! -f .env ]; then
    cp .env.example .env
    echo "Created .env file from .env.example"
fi

# Create necessary directories
mkdir -p logs
mkdir -p tmp

echo "Setup completed successfully!"
echo "Run 'make up' to start the services"