#!/bin/bash

# Dependency Security Audit Script
# This script checks for known vulnerabilities in Go dependencies

echo "🔍 Checking for vulnerable dependencies..."

# Install govulncheck if not available
if ! command -v govulncheck &> /dev/null; then
    echo "📦 Installing govulncheck..."
    go install golang.org/x/vuln/cmd/govulncheck@latest
fi

# Run vulnerability check
echo "🔍 Running vulnerability scan..."
govulncheck ./...

# Check for outdated dependencies
echo "📋 Checking for outdated dependencies..."
go list -u -m all | grep -v "^$" | sort

# Check for specific vulnerable packages mentioned in the audit
echo "⚠️  Checking for specific vulnerable packages..."

# modernc.org/sqlite v1.40.0 - Known vulnerability CVE-2023-7104
echo "Checking modernc.org/sqlite..."
current_sqlite=$(go list -m modernc.org/sqlite | awk '{print $2}')
echo "Current sqlite version: $current_sqlite"
if [[ "$current_sqlite" == "v1.40.0" ]]; then
    echo "⚠️  VULNERABILITY FOUND: modernc.org/sqlite v1.40.0 has CVE-2023-7104"
    echo "   Recommendation: Update to v1.41.0 or later"
fi

# golang.org/x/net v0.47.0 - Multiple vulnerabilities
echo "Checking golang.org/x/net..."
current_net=$(go list -m golang.org/x/net | awk '{print $2}')
echo "Current x/net version: $current_net"
if [[ "$current_net" == "v0.47.0" ]]; then
    echo "⚠️  VULNERABILITY FOUND: golang.org/x/net v0.47.0 has multiple vulnerabilities"
    echo "   Recommendation: Update to latest version"
fi

# github.com/charmbracelet/glamour v0.10.0 - XSS vulnerability
echo "Checking github.com/charmbracelet/glamour..."
current_glamour=$(go list -m github.com/charmbracelet/glamour | awk '{print $2}')
echo "Current glamour version: $current_glamour"
if [[ "$current_glamour" == "v0.10.0" ]]; then
    echo "⚠️  VULNERABILITY FOUND: github.com/charmbracelet/glamour v0.10.0 has XSS vulnerability"
    echo "   Recommendation: Update to v0.11.0 or later"
fi

echo "✅ Dependency audit complete!"
echo ""
echo "📋 Next steps:"
echo "1. Review the vulnerability scan results above"
echo "2. Update vulnerable dependencies using:"
echo "   go get -u [vulnerable-package]@latest"
echo "3. Run tests to ensure compatibility:"
echo "   make test"
echo "4. Consider using go mod tidy to clean up dependencies"