#!/bin/bash

# Deploy script for xiaomei backend
# Runs the main Python deployment script

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if we're in the right directory
if [ ! -f "config.yml" ]; then
    print_error "config.yml not found in current directory. Please run this script from the project root."
    exit 1
fi

# Check if Python is available
if ! command -v python3 &> /dev/null; then
    print_error "Python 3 is required but not installed."
    exit 1
fi

# Check if required Python packages are available
print_status "Checking Python dependencies..."
python3 -c "import yaml, pymongo, redis, requests, secrets" 2>/dev/null || {
    print_error "Required Python packages not found. Please install them:"
    echo "pip3 install pyyaml pymongo redis requests"
    exit 1
}
print_success "Python dependencies OK"

# Ask about production environment
echo -e "${YELLOW}Is this a production deployment? (y/N):${NC}"
read -r is_production

# Build command arguments
ARGS=""
if [[ $is_production =~ ^[Yy]$ ]]; then
    ARGS="--production"
    print_warning "Production deployment detected"
else
    print_status "Development deployment detected"
fi

# Ask about skipping database initialization
echo -e "${YELLOW}Skip database initialization? (y/N):${NC}"
read -r skip_db_init

if [[ $skip_db_init =~ ^[Yy]$ ]]; then
    ARGS="$ARGS --skip-db-init"
    print_status "Skipping database initialization"
fi

# Ask about skipping config generation
echo -e "${YELLOW}Skip config generation? (y/N):${NC}"
read -r skip_config

if [[ $skip_config =~ ^[Yy]$ ]]; then
    ARGS="$ARGS --skip-config"
    print_status "Skipping config generation"
fi

# Run the main Python deployment script
print_status "Running deployment script..."
python3 deploy/main.py $ARGS

print_success "Deployment completed!"
