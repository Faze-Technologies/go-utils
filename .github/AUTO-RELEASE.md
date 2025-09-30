# Manual Release Workflow

This repository uses GitHub Actions to create new releases when **manually triggered** from the GitHub UI.

## How It Works

### 🎯 **Manual Trigger Only**

- **When**: You manually trigger it from GitHub Actions tab
- **Where**: Go to Actions → "Auto Release" → "Run workflow"
- **What**: Creates tags, releases, and publishes to Go module registry

### 🚀 **How to Trigger a Release**

1. **Go to GitHub Actions**:
    - Navigate to your repository
    - Click on "Actions" tab
    - Find "Auto Release" workflow
    - Click "Run workflow"

2. **Choose Release Options**:
    - **Version Type**:
        - `auto` - Automatically detect from commit messages
        - `patch` - Force patch version (v1.0.7 → v1.0.8)
        - `minor` - Force minor version (v1.0.7 → v1.1.0)
        - `major` - Force major version (v1.0.7 → v2.0.0)
    - **Skip Version Check**:
        - `false` - (Default) Don't create if version already exists
        - `true` - Create release even if version tag exists

3. **Click "Run workflow"** 🚀

### 📊 **Version Detection Rules**

When using `auto` detection, the workflow analyzes commit messages:

| Commit Message Pattern                    | Version Bump | Example             |
|-------------------------------------------|--------------|---------------------|
| `feat:` or `feature:` or `add:` or `new:` | **MINOR**    | `v1.0.7` → `v1.1.0` |
| `fix:` or `bug:` or `patch:` or `hotfix:` | **PATCH**    | `v1.0.7` → `v1.0.8` |
| `BREAKING` or `!:` or `breaking`          | **MAJOR**    | `v1.0.7` → `v2.0.0` |

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

### 🎯 **What Gets Created**

1. **Version Calculation**: Determines next version based on commits or manual selection
2. **Git Tagging**: Creates and pushes version tags (e.g., `v1.1.0`)
3. **GitHub Release**: Creates release with auto-generated notes
4. **Go Module Publishing**: Package becomes available on `pkg.go.dev`
5. **Release Notes**: Generated from commit messages since last release

### 🔧 **Workflow Features**

- ✅ **Manual Control**: You decide when to release
- ✅ **Version Override**: Force specific version types
- ✅ **Smart Auto-Detection**: Analyzes commits when using 'auto'
- ✅ **Duplicate Prevention**: Optionally skip if version exists
- ✅ **Release Notes**: Auto-generated from commit history
- ✅ **Go Module Verification**: Tests module availability

### 📦 **After Release**

Once a new version is published:

```bash
# Users can immediately use your new version
go get github.com/Faze-Technologies/go-utils@v1.1.0

# Or get the latest
go get github.com/Faze-Technologies/go-utils@latest
```

### 🛠 **Example Scenarios**

#### **Scenario 1: Auto-detect from commits**

- Select: `auto` + `Skip version check: false`
- Result: Analyzes commits and creates appropriate version

#### **Scenario 2: Force a minor release**

- Select: `minor` + `Skip version check: false`
- Result: Creates v1.1.0 regardless of commit messages

#### **Scenario 3: Recreate existing version**

- Select: `patch` + `Skip version check: true`
- Result: Creates v1.0.8 even if it already exists

### 🔍 **Monitoring**

Check the Actions tab in GitHub to see:

- Version bump decisions
- Release creation status
- Any errors or issues

### 📋 **Best Practices**

1. **Use Auto-Detection**: Let the workflow analyze commits when possible
2. **Override When Needed**: Use manual version types for special releases
3. **Test Before Release**: Ensure your code works before triggering
4. **Review Generated Releases**: Check the created releases for accuracy

---

**Your New Workflow:**

1. ✅ **Develop** on feature branch
2. ✅ **Merge PR** to master
3. ✅ **Go to GitHub Actions**
4. ✅ **Click "Run workflow"** on "Auto Release"
5. ✅ **Choose version type** and click "Run workflow"
6. 🤖 **Everything else is automatic!**

**Example Workflow Output:**

```
🚀 Successfully released version v1.1.0
📈 Bump type: minor
🎯 Triggered manually with version type: auto
📦 Go module: github.com/Faze-Technologies/go-utils@v1.1.0
🔗 Release: https://github.com/Faze-Technologies/go-utils/releases/tag/v1.1.0
📚 Docs: https://pkg.go.dev/github.com/Faze-Technologies/go-utils@v1.1.0
```