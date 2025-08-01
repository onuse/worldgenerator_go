# Settings Configuration

The simulation can be configured via the `settings.json` file.

## Icosphere Resolution Levels

- Level 4: ~2,600 vertices (fast, low detail)
- Level 5: ~10,000 vertices (good balance)
- Level 6: ~40,000 vertices (high detail, default)
- Level 7: ~160,000 vertices (very high detail)
- Level 8: ~650,000 vertices (extreme detail, requires powerful GPU)

## Example settings.json

```json
{
  "simulation": {
    "icosphereLevel": 7,
    "comment": "Testing high resolution"
  },
  "server": {
    "port": 8080,
    "updateIntervalMs": 100
  },
  "gpu": {
    "preferMetal": true,
    "preferOpenCL": false
  }
}
```

## Notes

- Changes to `icosphereLevel` require a server restart
- Higher levels require more GPU memory and processing power
- The `updateIntervalMs` controls simulation frame rate (100ms = 10fps)