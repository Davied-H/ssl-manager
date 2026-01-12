# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
# Quick build for current platform
go build -o ssl-manager ./cmd/ssl-manager

# Build current platform with build script
./build.sh -c

# Build all platforms
./build.sh

# Build specific platform
./build.sh -p linux/amd64

# Release to GitHub + Docker Hub
./release.sh 0.0.1

# Release without Docker
./release.sh 0.0.1 --no-docker
```

## Run Commands

```bash
# Single run (check and apply certificates)
./ssl-manager config.yaml

# Daemon mode (background)
./ssl-manager config.yaml start
./ssl-manager config.yaml stop
./ssl-manager config.yaml status

# Foreground daemon (for debugging)
./ssl-manager config.yaml daemon

# Continue interrupted order
./ssl-manager config.yaml continue <orderID> <domain> [provider]
```

## Architecture

### Provider Pattern

The system uses a provider abstraction to support multiple cloud platforms:

- **CertProvider interface** (`internal/provider/cert.go`): Certificate operations (apply, download, status check)
- **DNSProvider interface** (`internal/provider/dns.go`): DNS record operations for domain validation

Implementations in `internal/provider/{aliyun,tencent,huawei}/`:
- Aliyun: Full support (cert + DNS)
- Tencent: Full support (cert + DNS)
- Huawei: DNS only (no API for free certificate application)

### Core Components

```
internal/
├── config/         # YAML config loading, provider/domain config structs
├── core/
│   ├── factory.go  # Provider factory with instance caching
│   ├── manager.go  # Main orchestrator: check -> apply -> validate -> download
│   ├── executor.go # Post-command execution with variable substitution
│   └── validator.go# Online certificate expiry checking
├── daemon/         # Background process management (PID file, signals)
├── domain/         # Domain name parsing utilities
├── provider/       # Provider interfaces and implementations
└── storage/        # Certificate file storage (PEM format)
```

### Certificate Flow

1. `Manager.ProcessDomain()` checks if valid cert exists on cloud platform
2. If not found or expiring, applies for new cert via `CertProvider.ApplyCertificate()`
3. `waitForDNSValidation()` adds TXT record via `DNSProvider`, polls until issued
4. Downloads cert and saves to `output_dir/<domain>/` (cert.pem, key.pem, fullchain.pem)
5. Executes optional `post_command` with variable substitution

### Mixed Provider Mode

Domain config supports using different providers for cert vs DNS:
```yaml
domains:
  - domain: "example.com"
    cert_provider: "tencent"  # Get cert from Tencent
    dns_provider: "aliyun"    # DNS validation via Aliyun
```

## Key Files

- `cmd/ssl-manager/main.go`: CLI entry point, command routing
- `internal/core/manager.go`: Main business logic
- `internal/core/factory.go`: Provider instantiation
- `config.yaml.example`: Configuration reference
