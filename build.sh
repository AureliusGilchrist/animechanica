#!/bin/bash

# Seanime Build Script
# This script builds both the web interface and the Go server

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WEB_DIR="$PROJECT_ROOT/seanime-web"
OUTPUT_DIR="$PROJECT_ROOT/web"
BUILD_TYPE="web"  # Default build type
PLATFORM=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

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

# Function to show usage
show_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -t, --type TYPE        Build type: web, desktop, denshi (default: web)"
    echo "  -p, --platform PLATFORM   Target platform: linux, windows, darwin (default: auto-detect)"
    echo "  -a, --arch ARCH        Target architecture: amd64, arm64 (default: auto-detect)"
    echo "  --web-only             Build only the web interface"
    echo "  --server-only          Build only the server"
    echo "  --no-systray           Build server without system tray support"
    echo "  --development          Build in development mode"
    echo "  -o, --output OUTPUT    Output binary name (default: seanime)"
    echo "  -h, --help             Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                     # Build web interface and server for current platform"
    echo "  $0 -t desktop          # Build desktop version"
    echo "  $0 --web-only          # Build only web interface"
    echo "  $0 --server-only       # Build only server"
    echo "  $0 -p windows          # Cross-compile for Windows"
}

# Parse command line arguments
WEB_ONLY=false
SERVER_ONLY=false
NO_SYSTRAY=false
DEVELOPMENT=false
OUTPUT_NAME="seanime"

while [[ $# -gt 0 ]]; do
    case $1 in
        -t|--type)
            BUILD_TYPE="$2"
            shift 2
            ;;
        -p|--platform)
            PLATFORM="$2"
            shift 2
            ;;
        -a|--arch)
            ARCH="$2"
            shift 2
            ;;
        --web-only)
            WEB_ONLY=true
            shift
            ;;
        --server-only)
            SERVER_ONLY=true
            shift
            ;;
        --no-systray)
            NO_SYSTRAY=true
            shift
            ;;
        --development)
            DEVELOPMENT=true
            shift
            ;;
        -o|--output)
            OUTPUT_NAME="$2"
            shift 2
            ;;
        -h|--help)
            show_usage
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

# Validate build type
case $BUILD_TYPE in
    web|desktop|denshi)
        ;;
    *)
        print_error "Invalid build type: $BUILD_TYPE. Must be one of: web, desktop, denshi"
        exit 1
        ;;
esac

# Set architecture mapping for Go
case $ARCH in
    x86_64)
        GOARCH="amd64"
        ;;
    aarch64|arm64)
        GOARCH="arm64"
        ;;
    *)
        GOARCH="amd64"  # Default fallback
        ;;
esac

# Set OS mapping for Go
case $PLATFORM in
    linux)
        GOOS="linux"
        BINARY_EXT=""
        ;;
    darwin)
        GOOS="darwin"
        BINARY_EXT=""
        ;;
    windows)
        GOOS="windows"
        BINARY_EXT=".exe"
        OUTPUT_NAME="${OUTPUT_NAME}.exe"
        ;;
    *)
        print_error "Unsupported platform: $PLATFORM"
        exit 1
        ;;
esac

print_status "Starting Seanime build process..."
print_status "Build type: $BUILD_TYPE"
print_status "Target platform: $GOOS/$GOARCH"
print_status "Output binary: $OUTPUT_NAME"

# Check prerequisites
check_prerequisites() {
    print_status "Checking prerequisites..."
    
    # Check Go version
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed or not in PATH"
        exit 1
    fi
    
    GO_VERSION=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
    REQUIRED_GO_VERSION="1.23"
    if ! printf '%s\n%s\n' "$REQUIRED_GO_VERSION" "$GO_VERSION" | sort -V -C; then
        print_error "Go version $GO_VERSION is too old. Required: $REQUIRED_GO_VERSION+"
        exit 1
    fi
    print_success "Go version $GO_VERSION is compatible"
    
    # Check Node.js and npm (only if building web)
    if [[ "$SERVER_ONLY" != true ]]; then
        if ! command -v node &> /dev/null; then
            print_error "Node.js is not installed or not in PATH"
            exit 1
        fi
        
        if ! command -v npm &> /dev/null; then
            print_error "npm is not installed or not in PATH"
            exit 1
        fi
        
        NODE_VERSION=$(node --version | sed 's/v//')
        REQUIRED_NODE_VERSION="18.0.0"
        if ! printf '%s\n%s\n' "$REQUIRED_NODE_VERSION" "$NODE_VERSION" | sort -V -C; then
            print_error "Node.js version $NODE_VERSION is too old. Required: $REQUIRED_NODE_VERSION+"
            exit 1
        fi
        print_success "Node.js version $NODE_VERSION is compatible"
    fi
}

# Build web interface
build_web() {
    print_status "Building web interface..."
    
    cd "$WEB_DIR"
    
    # Install dependencies if node_modules doesn't exist
    if [[ ! -d "node_modules" ]]; then
        print_status "Installing npm dependencies..."
        npm install
    fi
    
    # Determine build command based on type and development mode
    local build_cmd="npm run build"
    if [[ "$DEVELOPMENT" == true ]]; then
        case $BUILD_TYPE in
            desktop)
                build_cmd="npm run build"
                ;;
            *)
                build_cmd="npm run build"
                ;;
        esac
    else
        case $BUILD_TYPE in
            desktop)
                build_cmd="npm run build:desktop"
                ;;
            denshi)
                build_cmd="npm run build:denshi"
                ;;
            *)
                build_cmd="npm run build"
                ;;
        esac
    fi
    
    print_status "Running: $build_cmd"
    if ! $build_cmd; then
        print_error "Web build failed"
        exit 1
    fi
    
    # Move/copy build output to web directory
    if [[ -d "out" ]]; then
        # Static export build
        print_status "Moving web build output (static export)..."
        rm -rf "$OUTPUT_DIR"
        mv out "$OUTPUT_DIR"
        print_success "Web interface built successfully (export)"
    elif [[ -d ".next" ]]; then
        # Non-export build (SSR assets)
        print_status "Copying web build output (non-export)..."
        rm -rf "$OUTPUT_DIR"
        mkdir -p "$OUTPUT_DIR"
        cp -a .next "$OUTPUT_DIR/.next"
        # Include public assets if present
        if [[ -d "public" ]]; then
            cp -a public "$OUTPUT_DIR/public"
        fi
        # Include next.config for server awareness if needed
        if [[ -f "next.config.js" ]]; then
            cp next.config.js "$OUTPUT_DIR/next.config.js"
        fi
        print_success "Web interface built successfully (.next copied)"
    else
        print_error "Web build output not found (neither 'out' nor '.next' exists)"
        exit 1
    fi
    
    cd "$PROJECT_ROOT"
}

# Build Go server
build_server() {
    print_status "Building Go server..."
    
    cd "$PROJECT_ROOT"
    
    # Set build environment
    export CGO_ENABLED=1
    export GOOS="$GOOS"
    export GOARCH="$GOARCH"
    
    # Build flags
    local build_flags="-trimpath"
    local ldflags="-s -w"
    local tags=""
    
    # Platform-specific flags
    if [[ "$GOOS" == "windows" ]]; then
        if [[ "$NO_SYSTRAY" != true && "$BUILD_TYPE" != "desktop" ]]; then
            ldflags="$ldflags -H=windowsgui -extldflags '-static'"
        fi
    fi
    
    # Add no-systray tag if specified or for desktop builds
    if [[ "$NO_SYSTRAY" == true || "$BUILD_TYPE" == "desktop" ]]; then
        tags="-tags=nosystray"
    fi
    
    # Cross-compilation setup
    if [[ "$GOOS" != "$(go env GOOS)" || "$GOARCH" != "$(go env GOARCH)" ]]; then
        print_status "Cross-compiling for $GOOS/$GOARCH"
        case "$GOOS" in
            windows)
                if command -v x86_64-w64-mingw32-gcc &> /dev/null; then
                    export CC=x86_64-w64-mingw32-gcc
                    export CXX=x86_64-w64-mingw32-g++
                fi
                ;;
        esac
    fi
    
    # Build command
    local build_cmd="go build -o $OUTPUT_NAME $build_flags -ldflags=\"$ldflags\" $tags"
    print_status "Running: $build_cmd"
    
    if ! eval $build_cmd; then
        print_error "Server build failed"
        exit 1
    fi
    
    print_success "Server built successfully: $OUTPUT_NAME"
    
    # Show binary info
    if [[ -f "$OUTPUT_NAME" ]]; then
        local file_size=$(du -h "$OUTPUT_NAME" | cut -f1)
        print_status "Binary size: $file_size"
        
        if command -v file &> /dev/null; then
            print_status "Binary info: $(file "$OUTPUT_NAME")"
        fi
    fi
}

# Clean build artifacts
clean_build() {
    print_status "Cleaning previous build artifacts..."
    rm -rf "$OUTPUT_DIR"
    rm -f seanime seanime.exe
    if [[ -d "$WEB_DIR/out" ]]; then
        rm -rf "$WEB_DIR/out"
    fi
    if [[ -d "$WEB_DIR/.next" ]]; then
        rm -rf "$WEB_DIR/.next"
    fi
}

# Main build process
main() {
    # Check if we're in the right directory
    if [[ ! -f "main.go" ]]; then
        print_error "main.go not found. Please run this script from the project root directory."
        exit 1
    fi
    
    check_prerequisites
    
    # Clean previous builds
    clean_build
    
    # Build based on options
    if [[ "$SERVER_ONLY" == true ]]; then
        build_server
    elif [[ "$WEB_ONLY" == true ]]; then
        build_web
    else
        build_web
        build_server
    fi
    
    print_success "Build completed successfully!"
    
    # Show next steps
    echo ""
    print_status "Next steps:"
    if [[ "$WEB_ONLY" != true ]]; then
        echo "  - Run the server: ./$OUTPUT_NAME"
    fi
    if [[ "$SERVER_ONLY" != true ]]; then
        echo "  - Web interface built in: $OUTPUT_DIR"
    fi
    echo "  - Check the logs for any runtime issues"
}

# Run main function
main "$@"
