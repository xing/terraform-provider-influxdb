# Release Process

Detailed technical guide for creating releases of the Terraform InfluxDB Provider.

## Setup Requirements

### 1. Install Tools
```bash
# macOS
brew install goreleaser gh

# Linux (check goreleaser.com for other methods)
curl -sfL https://install.goreleaser.com/github.com/goreleaser/goreleaser.sh | sh
```

### 2. Configure GPG Key
```bash
# Generate new key (if needed)
gpg --gen-key

# List keys to get fingerprint
gpg --list-secret-keys --keyid-format LONG

# Export public key for Terraform Registry
gpg --armor --export YOUR_KEY_ID
```

### 3. GitHub Authentication
```bash
gh auth login --git-protocol https --scopes repo,workflow
```

## Quick Release
```bash
make release-notes VERSION=v0.1.8    # Edit RELEASE_NOTES.md  
git add . && git commit -m "Release v0.1.8" && git tag v0.1.8
export GPG_FINGERPRINT=YOUR_KEY_FINGERPRINT
make goreleaser-release VERSION=v0.1.8
```

## GoReleaser Output

GoReleaser creates releases with:
- **14+ platform binaries** (Linux, macOS, Windows, FreeBSD × amd64/arm64/386/arm)
- **Proper ZIP archives** with correct binary names inside
- **SHA256SUMS** and **SHA256SUMS.sig** (binary GPG signature) 
- **manifest.json** with Terraform protocol version
- **Terraform Registry compatible** naming and structure

## Registry Submission

After release, submit to Terraform Registry:
1. Visit https://registry.terraform.io/publish/provider
2. Sign in with GitHub account that has `xing` org access
3. Select `xing/terraform-provider-influxdb`
4. Registry auto-detects latest release

## Troubleshooting

### GPG Issues

If you get GPG errors:

```bash
# List your GPG keys
gpg --list-secret-keys

# Get the fingerprint (long format)
gpg --list-keys --keyid-format LONG

# Test signing
echo "test" | gpg --armor --detach-sign
```

### GitHub Token Issues

```bash
# Check authentication
gh auth status

# Re-authenticate if needed
gh auth login --git-protocol https --scopes repo,workflow
```

### GoReleaser Configuration

The `.goreleaser.yml` file is based on HashiCorp's official template and should not be modified unless necessary. It ensures compatibility with the Terraform Registry.

## Testing & Validation

```bash
# Test build without release
make goreleaser-build

# Validate .goreleaser.yml
goreleaser check

# Preview release (no upload)
goreleaser release --snapshot --clean
```

## Common Issues & Solutions

### "Missing platforms" in Registry
- Ensure using GoReleaser (not manual Makefile builds)
- Check binary names inside ZIP files match: `terraform-provider-influxdb_vX.X.X`
- Verify archive names: `terraform-provider-influxdb_X.X.X_{os}_{arch}.zip`

### GPG Signing Failures
```bash
# Test GPG setup
echo "test" | gpg --armor --detach-sign

# Cache passphrase (if using one)
echo "dummy" | gpg --sign --batch --yes --passphrase-fd 0
```

### GitHub API Issues
```bash
# Check token scopes
gh auth status

# Refresh token
gh auth refresh --scopes repo,workflow
```