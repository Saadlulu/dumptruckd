# Configuration Files Guide

## Which Config File to Use?

The **actual config file** that dumptruckd uses is: **`config/dumptruckd.toml`**

This file doesn't exist by default - you need to create it from one of the templates below.

## Configuration Templates

### 1. `dumptruckd.toml.example` (Recommended - Modular Setup)

**Use this for:** Production setups, multiple databases, reusable components

**Setup:**
```bash
cp config/dumptruckd.toml.example config/dumptruckd.toml
```

**How it works:**
- Main file references component files in `config.d/`
- Components are defined in separate files
- Easy to manage and reuse components
- Best for complex setups

**Files structure:**
```
config/
├── dumptruckd.toml          # Main config (you create this)
└── config.d/
    ├── databases.toml       # Database components
    ├── uploaders.toml       # Uploader components
    ├── compressors.toml     # Compressor components
    ├── retention.toml       # Retention components
    └── backups.toml         # Backup job definitions
```

### 2. `example-single-file.toml` (Alternative - Single File)

**Use this for:** Simple setups, quick testing, everything in one place

**Setup:**
```bash
cp config/example-single-file.toml config/dumptruckd.toml
```

**How it works:**
- Everything defined in one file
- Can still use component references
- Simpler for small setups
- Good for testing

## Quick Start

```bash
# Choose your approach:

# Modular (recommended)
cp config/dumptruckd.toml.example config/dumptruckd.toml
# Then edit config.d/*.toml files

# OR

# Single file
cp config/example-single-file.toml config/dumptruckd.toml
# Then edit the single file
```

## File Summary

| File | Purpose | When to Use |
|------|---------|-------------|
| `dumptruckd.toml` | **Actual config file** | Always (you create this) |
| `dumptruckd.toml.example` | Template for modular setup | Copy to create `dumptruckd.toml` |
| `example-single-file.toml` | Template for single-file setup | Copy to create `dumptruckd.toml` |
| `config.d/*.toml` | Component definitions | Used by modular setup |

## Default Behavior

If you don't specify `-config`, dumptruckd looks for:
1. `/etc/dumptruckd/dumptruckd.toml`
2. `config/dumptruckd.toml`
3. `dumptruckd.toml`

You can override with:
```bash
dumptruckd -config /path/to/your/config.toml
```

See [../docs/CONFIGURATION.md](../docs/CONFIGURATION.md) for the full configuration guide.
