# Resizr Migration Tool

This migration tool rebuilds atomic resolution references for existing images in the system. It's designed to be run once when upgrading from the old reference tracking system to the new atomic reference counting system.

## Purpose

The migration tool:
- Scans all existing images in the repository
- Extracts resolution information from each image's metadata
- Populates atomic resolution references using `AddResolutionRef()`
- Supports both Redis and Badger backends
- Provides progress tracking and statistics

## Usage

### Basic Migration

```bash
# Build the migration tool
go build -o migrate ./cmd/migrate

# Run migration (makes actual changes)
./migrate
```

### Dry Run Mode

Test the migration without making any changes:

```bash
./migrate -dry-run
```

### Custom Batch Size

Process images in smaller or larger batches:

```bash
# Process 50 images at a time
./migrate -batch-size 50

# Process 500 images at a time (faster but more memory)
./migrate -batch-size 500
```

### Combined Options

```bash
# Dry run with custom batch size
./migrate -dry-run -batch-size 200
```

## Command-Line Flags

- `-dry-run`: Run in dry-run mode (no changes made). Default: `false`
- `-batch-size`: Number of images to process per batch. Default: `100`

## Configuration

The migration tool reads configuration from the same `.env` file as the main application:

Required environment variables:
- `CACHE_TYPE`: Must be either `redis` or `badger`
- For Redis: `REDIS_ADDR`, `REDIS_PASSWORD`, `REDIS_DB`
- For Badger: `CACHE_DIRECTORY`

Example `.env`:
```bash
CACHE_TYPE=redis
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
```

## Output

The migration tool provides:

### Progress Logging
```
INFO    Starting migration      {"app": "Resizr Migration Tool", "version": "0.0.1", "dry_run": false, "batch_size": 100}
INFO    Fetching all images from repository
INFO    Found images to process {"count": 1523}
INFO    Processing batch        {"batch_start": 1, "batch_end": 100, "total": 1523}
INFO    Processing batch        {"batch_start": 101, "batch_end": 200, "total": 1523}
...
```

### Final Statistics
```
INFO    Migration completed successfully
        {
          "total_images": 1523,
          "total_resolutions": 4569,
          "refs_created": 4569,
          "errors": 0,
          "duration": "45.3s",
          "dry_run": false
        }

✓ Migration completed successfully

Statistics:
  Total images processed: 1523
  Total resolutions: 4569
  Reference entries created: 4569
  Errors encountered: 0
  Duration: 45.3s
```

## Migration Process

For each image, the tool:

1. Fetches image metadata from the repository
2. Skips images without a hash (legacy non-deduplicated images without deduplication tracking)
3. Extracts all resolutions (original + generated resolutions like "thumbnail", "1920x1080", etc.)
4. Calls `AddResolutionRef(ctx, hash, resolution, imageID)` for each resolution
5. Logs progress and any errors

## Error Handling

- **Individual Image Errors**: If an image fails to process, the error is logged and the migration continues with the next image
- **Fatal Errors**: Repository connection failures or configuration errors will stop the migration
- **Partial Completion**: If the migration is interrupted, it's safe to run again - `AddResolutionRef()` is idempotent (adding the same reference twice has no negative effect with Redis Sets or Badger's array-based storage)

## When to Run

Run this migration:
- **Once** after deploying the new atomic reference counting system
- Before deploying workers that depend on atomic resolution references
- During a maintenance window for large datasets (recommended but not required)

## Performance

Expected performance:
- **Small datasets** (<1000 images): 10-30 seconds
- **Medium datasets** (1000-10000 images): 1-5 minutes
- **Large datasets** (>10000 images): 5-30 minutes

Performance factors:
- Backend type (Badger is typically faster for bulk operations)
- Network latency (Redis over network vs Badger local disk)
- Number of resolutions per image
- Batch size setting

## Safety

The migration is safe to run multiple times:
- Uses atomic `AddResolutionRef()` which is idempotent
- No data is deleted or modified, only new reference entries are created
- Original deduplication info remains unchanged
- Dry-run mode available for testing

## Verification

After running the migration, verify success:

1. Check migration output for errors
2. Verify reference counts match expected values:
   ```bash
   # For Redis
   redis-cli --scan --pattern "dedup:res_refs:*" | head -10
   redis-cli SMEMBERS "dedup:res_refs:HASH:original"

   # Check count
   redis-cli SCARD "dedup:res_refs:HASH:original"
   ```

3. Test deletion operations to ensure atomic reference counting works
4. Monitor logs during normal operation for any reference count warnings

## Troubleshooting

### "Repository does not implement DeduplicationRepository interface"
- Ensure you're using a compatible repository version
- Check that `CACHE_TYPE` is set to either `redis` or `badger`

### "Failed to list images at offset X"
- Check repository connection
- Verify sufficient memory for large datasets
- Try reducing `-batch-size`

### High Memory Usage
- Reduce `-batch-size` to process fewer images at once
- For very large datasets (>100k images), consider running in multiple passes with filtered image ID ranges

### Migration Taking Too Long
- Increase `-batch-size` for faster processing
- Ensure backend is not under heavy load
- For Redis, consider using a local Redis instance temporarily
- For Badger, ensure SSD storage is used

## Post-Migration

After successful migration:
1. Deploy the updated application with atomic deletion operations
2. Start deletion workers if using async deletion mode
3. Monitor Prometheus metrics for reference count operations
4. The migration tool can be removed or kept for future reference rebuilds

## Development

To modify the migration logic:
1. Edit `cmd/migrate/main.go`
2. Rebuild: `go build -o migrate ./cmd/migrate`
3. Test with `-dry-run` first
4. Run migration

## Support

For issues or questions:
- Check application logs in the migration output
- Verify configuration in `.env`
- Review repository connectivity
- Consult the main Resizr documentation
