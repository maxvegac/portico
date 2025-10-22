#!/bin/bash

# Portico Version Management Script

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Get current version from git tags
get_current_version() {
    local version=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
    echo "${version#v}"
}

# Get next version
get_next_version() {
    local current=$(get_current_version)
    local major=$(echo $current | cut -d. -f1)
    local minor=$(echo $current | cut -d. -f2)
    local patch=$(echo $current | cut -d. -f3)
    
    echo "$major.$minor.$((patch + 1))"
}

# Get dev version
get_dev_version() {
    local current=$(get_current_version)
    local commit=$(git rev-parse --short HEAD)
    echo "${current}-dev-${commit}"
}

# Create a new release
create_release() {
    local version=$1
    local prerelease=${2:-false}
    
    echo -e "${BLUE}📦 Creating release v$version${NC}"
    
    # Create and push tag
    git tag "v$version"
    git push origin "v$version"
    
    echo -e "${GREEN}✅ Release v$version created!${NC}"
    echo -e "${BLUE}🔗 Check: https://github.com/portico/portico/releases${NC}"
}

# Create a dev release
create_dev_release() {
    local version=$(get_dev_version)
    
    echo -e "${BLUE}📦 Creating dev release v$version${NC}"
    
    # Create and push tag
    git tag "v$version"
    git push origin "v$version"
    
    echo -e "${GREEN}✅ Dev release v$version created!${NC}"
    echo -e "${BLUE}🔗 Check: https://github.com/portico/portico/releases${NC}"
}

# Show current version info
show_version() {
    local current=$(get_current_version)
    local next=$(get_next_version)
    local dev=$(get_dev_version)
    
    echo -e "${BLUE}📋 Version Information:${NC}"
    echo -e "  Current:  v$current"
    echo -e "  Next:     v$next"
    echo -e "  Dev:      v$dev"
    echo ""
    echo -e "${BLUE}🔧 Commands:${NC}"
    echo -e "  $0 release [version]  - Create stable release"
    echo -e "  $0 dev               - Create dev release"
    echo -e "  $0 patch             - Create patch release"
    echo -e "  $0 minor             - Create minor release"
    echo -e "  $0 major             - Create major release"
}

# Main function
main() {
    case "${1:-help}" in
        "release")
            local version=${2:-$(get_next_version)}
            create_release "$version" false
            ;;
        "dev")
            create_dev_release
            ;;
        "patch")
            local current=$(get_current_version)
            local major=$(echo $current | cut -d. -f1)
            local minor=$(echo $current | cut -d. -f2)
            local patch=$(echo $current | cut -d. -f3)
            create_release "$major.$minor.$((patch + 1))" false
            ;;
        "minor")
            local current=$(get_current_version)
            local major=$(echo $current | cut -d. -f1)
            local minor=$(echo $current | cut -d. -f2)
            create_release "$major.$((minor + 1)).0" false
            ;;
        "major")
            local current=$(get_current_version)
            local major=$(echo $current | cut -d. -f1)
            create_release "$((major + 1)).0.0" false
            ;;
        "help"|*)
            show_version
            ;;
    esac
}

main "$@"
