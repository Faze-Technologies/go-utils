# Auto-Release Workflow

This repository uses GitHub Actions to automatically create new releases when PRs are merged to the master branch.

## How It Works

### 🚀 Automatic Triggers
- **When**: PR is merged to `master` or direct push to `master`
- **What**: Analyzes commits, determines version bump, creates tag, and publishes release

### 📊 Version Bump Rules

The workflow analyzes commit messages to determine the version bump type:

| Commit Message Pattern | Version Bump | Example |
|------------------------|--------------|---------|
| `feat:` or `feature:` or `add:` or `new:` | **MINOR** | `v1.0.7` → `v1.1.0` |
| `fix:` or `bug:` or `patch:` or `hotfix:` | **PATCH** | `v1.0.7` → `v1.0.8` |
| `BREAKING` or `!:` or `breaking` | **MAJOR** | `v1.0.7` → `v2.0.0` |

### 📝 Commit Message Examples

#### Minor Version (New Features)
```
feat: Add new Redis caching utility
feature: Implement JWT authentication middleware
add: New MongoDB connection helper
```

#### Patch Version (Bug Fixes)
```
fix: Resolve memory leak in Redis client
bug: Fix null pointer in logger
patch: Update dependency versions
```

#### Major Version (Breaking Changes)
```
feat!: Remove deprecated config methods
BREAKING: Change function signatures in auth module
fix!: Remove legacy database connections
```

### 🎯 What Gets Automated

1. **Version Calculation**: Automatically determines next version based on commits
2. **Git Tagging**: Creates and pushes version tags (e.g., `v1.1.0`)
3. **GitHub Release**: Creates release with auto-generated notes
4. **Go Module Publishing**: Package becomes available on `pkg.go.dev`
5. **Release Notes**: Generated from commit messages since last release

### 🔧 Workflow Features

- ✅ **Smart Version Detection**: Analyzes all commits since last tag
- ✅ **Duplicate Prevention**: Skips if version already exists
- ✅ **Release Notes**: Auto-generated from commit history
- ✅ **Go Module Verification**: Tests module availability
- ✅ **Detailed Logging**: Shows version bump reasoning

### 📦 After Release

Once a new version is published:

```bash
# Users can immediately use your new version
go get github.com/Faze-Technologies/go-utils@v1.1.0

# Or get the latest
go get github.com/Faze-Technologies/go-utils@latest
```

### 🛠 Manual Override

If you need to create a release manually:

```bash
# Create tag locally
git tag v1.1.0
git push origin v1.1.0

# The workflow will still create a GitHub release
```

### 🔍 Monitoring

Check the Actions tab in GitHub to see:
- Version bump decisions
- Release creation status
- Any errors or issues

### 📋 Best Practices

1. **Use Conventional Commits**: Follow the patterns above for automatic versioning
2. **Meaningful Messages**: Write clear commit messages for better release notes
3. **Test Before Merge**: Ensure your code works before merging to master
4. **Review Releases**: Check the generated releases for accuracy

---

**Example Workflow Output:**
```
🚀 Successfully released version v1.1.0
📈 Bump type: minor
📦 Go module: github.com/Faze-Technologies/go-utils@v1.1.0
🔗 Release: https://github.com/Faze-Technologies/go-utils/releases/tag/v1.1.0
📚 Docs: https://pkg.go.dev/github.com/Faze-Technologies/go-utils@v1.1.0
```