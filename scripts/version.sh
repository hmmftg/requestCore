#!/bin/bash

# Automatic Semantic Versioning Script
# This script helps manage semantic versioning for the requestCore project

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
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
    echo "Usage: $0 [COMMAND] [OPTIONS]"
    echo ""
    echo "Commands:"
    echo "  current          Show current version"
    echo "  next [type]      Show next version (major|minor|patch)"
    echo "  bump [type]      Bump version and create tag (major|minor|patch)"
    echo "  release [type]   Create release with version bump"
    echo "  check            Check if version needs bumping"
    echo "  validate         Validate commit messages for conventional commits"
    echo ""
    echo "Options:"
    echo "  --dry-run        Show what would be done without executing"
    echo "  --help           Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 current"
    echo "  $0 next minor"
    echo "  $0 bump patch"
    echo "  $0 release major --dry-run"
}

# Function to get current version
get_current_version() {
    local latest_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
    echo "${latest_tag#v}"
}

# Function to get next version
get_next_version() {
    local bump_type=$1
    local current_version=$(get_current_version)
    local IFS='.'
    read -r major minor patch <<< "$current_version"
    
    case $bump_type in
        major)
            major=$((major + 1))
            minor=0
            patch=0
            ;;
        minor)
            minor=$((minor + 1))
            patch=0
            ;;
        patch)
            patch=$((patch + 1))
            ;;
        *)
            print_error "Invalid bump type: $bump_type. Use major, minor, or patch."
            exit 1
            ;;
    esac
    
    echo "v${major}.${minor}.${patch}"
}

# Function to analyze commits and suggest version bump
analyze_commits() {
    local latest_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
    local commits=$(git log --pretty=format:"%s" ${latest_tag}..HEAD 2>/dev/null || echo "")
    
    if [ -z "$commits" ]; then
        echo "patch"
        return
    fi
    
    # Check for breaking changes
    if echo "$commits" | grep -qiE "(breaking|!|BREAKING)"; then
        echo "major"
        return
    fi
    
    # Check for new features
    if echo "$commits" | grep -qiE "^feat|^feature|^add"; then
        echo "minor"
        return
    fi
    
    # Check for fixes, docs, refactor, etc.
    if echo "$commits" | grep -qiE "^feat|^fix|^bug|^patch|^docs|^style|^refactor|^perf|^test|^chore"; then
        echo "patch"
        return
    fi
    
    # Default to patch
    echo "patch"
}

# Function to validate commit messages
validate_commits() {
    local latest_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
    local commits=$(git log --pretty=format:"%h - %s" ${latest_tag}..HEAD 2>/dev/null || echo "")
    
    if [ -z "$commits" ]; then
        print_info "No commits since last tag: $latest_tag"
        return 0
    fi
    
    print_info "Validating commits since $latest_tag:"
    echo ""
    
    local invalid_commits=0
    while IFS= read -r commit; do
        local commit_msg=$(echo "$commit" | cut -d' ' -f3-)
        if echo "$commit_msg" | grep -qE "^(feat|fix|docs|style|refactor|perf|test|chore|build|ci|revert)(\(.+\))?: .+"; then
            echo -e "  ${GREEN}✓${NC} $commit"
        else
            echo -e "  ${RED}✗${NC} $commit"
            invalid_commits=$((invalid_commits + 1))
        fi
    done <<< "$commits"
    
    echo ""
    if [ $invalid_commits -eq 0 ]; then
        print_success "All commits follow conventional commit format!"
    else
        print_warning "$invalid_commits commit(s) don't follow conventional commit format"
        print_info "Conventional commit format: <type>(<scope>): <description>"
        print_info "Types: feat, fix, docs, style, refactor, perf, test, chore, build, ci, revert"
    fi
}

# Function to bump version
bump_version() {
    local bump_type=$1
    local dry_run=$2
    local new_version=$(get_next_version "$bump_type")
    local current_version=$(get_current_version)
    
    if [ "$new_version" = "v$current_version" ]; then
        print_warning "Version is already at $new_version"
        return 0
    fi
    
    print_info "Bumping version from v$current_version to $new_version"
    
    if [ "$dry_run" = "true" ]; then
        print_info "[DRY RUN] Would create tag: $new_version"
        print_info "[DRY RUN] Would push tag to origin"
        return 0
    fi
    
    # Create and push tag
    git tag -a "$new_version" -m "Release $new_version"
    git push origin "$new_version"
    
    print_success "Created and pushed tag: $new_version"
}

# Function to create release
create_release() {
    local bump_type=$1
    local dry_run=$2
    local new_version=$(get_next_version "$bump_type")
    local current_version=$(get_current_version)
    
    if [ "$new_version" = "v$current_version" ]; then
        print_warning "Version is already at $new_version"
        return 0
    fi
    
    print_info "Creating release $new_version"
    
    if [ "$dry_run" = "true" ]; then
        print_info "[DRY RUN] Would create release: $new_version"
        return 0
    fi
    
    # Create release using GitHub CLI if available
    if command -v gh &> /dev/null; then
        local latest_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
        local commits=$(git log --pretty=format:"- %s (%h)" ${latest_tag}..HEAD 2>/dev/null || echo "- Initial release")
        
        gh release create "$new_version" \
            --title "Release $new_version" \
            --notes "## Changes in this release

### Commits since $latest_tag:
$commits

### Version Bump Type
- **$(echo $bump_type | tr '[:lower:]' '[:upper:]')** version increment"
        
        print_success "Created release: $new_version"
    else
        print_warning "GitHub CLI (gh) not found. Please install it to create releases automatically."
        print_info "You can create the release manually at: https://github.com/hmmftg/requestCore/releases/new"
    fi
}

# Main script logic
main() {
    local command=$1
    local option=$2
    local dry_run="false"
    
    # Parse options
    if [ "$2" = "--dry-run" ] || [ "$3" = "--dry-run" ]; then
        dry_run="true"
    fi
    
    case $command in
        current)
            print_info "Current version: v$(get_current_version)"
            ;;
        next)
            if [ -z "$option" ]; then
                print_error "Please specify bump type: major, minor, or patch"
                exit 1
            fi
            local next_version=$(get_next_version "$option")
            print_info "Next version: $next_version"
            ;;
        bump)
            if [ -z "$option" ]; then
                print_error "Please specify bump type: major, minor, or patch"
                exit 1
            fi
            bump_version "$option" "$dry_run"
            ;;
        release)
            if [ -z "$option" ]; then
                print_error "Please specify bump type: major, minor, or patch"
                exit 1
            fi
            bump_version "$option" "$dry_run"
            create_release "$option" "$dry_run"
            ;;
        check)
            local suggested_bump=$(analyze_commits)
            print_info "Suggested version bump: $suggested_bump"
            local next_version=$(get_next_version "$suggested_bump")
            print_info "Next version would be: $next_version"
            ;;
        validate)
            validate_commits
            ;;
        --help|help)
            show_usage
            ;;
        *)
            print_error "Unknown command: $command"
            show_usage
            exit 1
            ;;
    esac
}

# Run main function with all arguments
main "$@"
