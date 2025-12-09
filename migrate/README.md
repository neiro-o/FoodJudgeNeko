# MongoDB Migration Script

This script migrates data from a source MongoDB database to the target MongoDB database configured in `config.yml`.

## Features

- **SSH Tunnel Support**: Connect to source MongoDB through SSH tunnel
- **Flexible Export**: Export full data or a percentage of data
- **Batch Processing**: Efficient batch processing for large datasets
- **Dry Run Mode**: Test migration without actually migrating data

## Setup

1. Install required dependencies:
```bash
pip install pymongo pyyaml sshtunnel
```

Or add to your requirements file:
```
pymongo>=4.6.0
pyyaml>=6.0
sshtunnel>=0.4.0
```

2. Copy the example config file:
```bash
cp migrate_config.yml.example migrate_config.yml
```

3. Edit `migrate_config.yml` with your source MongoDB connection details.

## Configuration

### Source MongoDB Configuration

The `migrate_config.yml` file contains:

- **SSH Tunnel Settings** (optional):
  - `enabled`: Set to `true` to enable SSH tunneling
  - `host`: SSH server hostname
  - `port`: SSH server port (default: 22)
  - `username`: SSH username
  - Authentication: Use one of:
    - `password`: SSH password
    - `key_file`: Path to SSH private key file
    - `key_string`: SSH private key as string

- **MongoDB Connection**:
  - `connection_string`: MongoDB connection string (direct format)
    - If SSH is enabled: Set to `mongodb://127.0.0.1:LOCAL_PORT` where LOCAL_PORT matches your SSH tunnel's `local_port` setting (e.g., `mongodb://127.0.0.1:27018`)
    - If SSH is disabled: Use the direct connection string to your MongoDB server (e.g., `mongodb://host:port`)
    - Authentication can be included in the connection string: `mongodb://username:password@host:port`
  - `database_name`: Source database name (default: `mtdb`)
  - `collections`: Source collection names
    - `meituan`: Collection to migrate to `problems`
    - `manual`: Collection to migrate to `notes`

### Migration Options

- `mode`: `"full"` or `"percentile"`
- `percentile`: Percentage to export (0-100, only used if mode is "percentile")
- `batch_size`: Number of documents to process per batch (default: 1000)

## Usage

### Basic Migration

```bash
python migrate.py
```

### Custom Config Files

```bash
python migrate.py --config custom_migrate_config.yml --target-config ../config.yml
```

### Dry Run (Test Without Migrating)

```bash
python migrate.py --dry-run
```

## Migration Mapping

- Source `meituan` collection → Target `problems` collection
- Source `manual` collection → Target `notes` collection

The target database and collections are read from `config.yml`.

## Examples

### Example 1: Full Migration with SSH

1. Edit `migrate_config.yml`:
```yaml
source:
  ssh:
    enabled: true
    host: "remote-server.com"
    username: "user"
    key_file: "~/.ssh/id_rsa"
    local_port: 27018
  connection_string: "mongodb://username:password@127.0.0.1:27018"
  database_name: "mtdb"
migration:
  mode: "full"
```

Note: The `connection_string` points to `127.0.0.1:27018` which matches the SSH tunnel's `local_port`.

2. Run migration:
```bash
python migrate.py
```

### Example 2: Partial Migration (50%)

1. Edit `migrate_config.yml`:
```yaml
migration:
  mode: "percentile"
  percentile: 50
```

2. Run migration:
```bash
python migrate.py
```

### Example 3: Direct Connection (No SSH)

1. Edit `migrate_config.yml`:
```yaml
source:
  ssh:
    enabled: false
  connection_string: "mongodb://source-server:27017"
  database_name: "mtdb"
```

2. Run migration:
```bash
python migrate.py
```

## Notes

- The script handles duplicate documents gracefully (skips them)
- Large datasets are processed in batches for efficiency
- Percentile mode uses random sampling
- The script preserves document structure and IDs
- Make sure you have read access to source database and write access to target database
