# cron-manager

cron-manager is a Go tool for wrapping and monitoring cron jobs, publishing metrics through Prometheus Node Exporter's TextFile Collector.

## Features

- Execute cron job commands and monitor their status
- Measure job execution time
- Detect job failures and timeouts
- Publish monitoring metrics via Prometheus TextFile Collector
- Support custom metrics file path
- Support job output logging
- Support idle wait mode (for Prometheus to detect job running status)

## Installation

### Build

```bash
make build
```

### Install to System Path

```bash
sudo mv ./bin/cronmanager /usr/local/bin/
```

## Usage

### Basic Usage

```bash
cronmanager -n "job_name" [options] -- command [args...]
```

### Examples

```bash
# Execute PHP script
cronmanager -n "task_cron" -- /usr/bin/php /var/www/app/console task:run

# Execute Python script with logging
cronmanager -n "script_cron" -l /var/log/cron.log -- /usr/bin/python3 /path/to/script.py

# Enable idle wait mode (wait at least 60 seconds)
cronmanager -n "job_cron" -i 60 -- /usr/bin/command arg1 arg2

# Use custom Prometheus exporter directory
cronmanager -n "job_cron" -d /tmp/prometheus -- /usr/bin/command

# Use custom Prometheus exporter filename
cronmanager -n "job_cron" --textfile my-metrics.prom -- /usr/bin/command

# Use custom directory and filename
cronmanager -n "job_cron" -d /tmp/prometheus --textfile custom.prom -- /usr/bin/command

# Custom idle wait duration (wait at least 120 seconds)
cronmanager -n "job_cron" -i 120 -- /usr/bin/command arg1 arg2

# Command with complex arguments (no need to escape)
cronmanager -n "update_cron" -- /usr/bin/php /var/www/app/console broadcast:entities:updated -e project -l 20000
```

### Command Line Options

| Option | Description | Required | Default |
|--------|-------------|----------|---------|
| `-n` | Job name (will appear in alerts) | ✅ | "Generic" |
| `-l` | Log file path | ❌ | None (output will be discarded) |
| `-i` | Idle wait duration in seconds (ensures job runs for at least this duration for Prometheus detection) | ❌ | 0 (disabled) |
| `-d` | Directory for Prometheus exporter file | ❌ | `/var/cache/prometheus` or `COLLECTOR_TEXTFILE_PATH` env var |
| `--textfile` | Filename for Prometheus exporter file | ❌ | `crons.prom` |
| `-version` | Display version information and exit | ❌ | - |
| `--` | Separator before command and its arguments | ✅ | - |

### Notes

- The command and its arguments must be placed after the `--` separator
- This syntax allows you to use commands with complex arguments without escaping
- The command must be an executable file path; shell built-ins or pipe operations are not supported
- It's recommended to append `_cron` suffix to job names for easier identification in Prometheus/Grafana

## Configuration

### Prometheus Node Exporter

Ensure Prometheus Node Exporter is installed and configured with TextFile Collector enabled:

```bash
node_exporter \
  --collector.textfile \
  --collector.textfile.directory=/var/cache/prometheus
```

### Custom Metrics File Path

You can customize both the directory and filename for the Prometheus exporter file:

**Directory** (priority order):
1. **Command line argument `-d`** (highest priority):
```bash
cronmanager -n "job_cron" -d /custom/path/to/directory -- command
```

2. **Environment variable `COLLECTOR_TEXTFILE_PATH`**:
```bash
export COLLECTOR_TEXTFILE_PATH=/custom/path/to/directory
cronmanager -n "job_cron" -- command
```

3. **Default path** (lowest priority): `/var/cache/prometheus`

**Filename**:
- Use `--textfile` to specify a custom filename (default: `crons.prom`):
```bash
cronmanager -n "job_cron" --textfile my-metrics.prom -- command
```

**Combined example**:
```bash
cronmanager -n "job_cron" -d /tmp/prometheus --textfile custom.prom -- command
```

### Permissions

Ensure the user running cron-manager has write permissions to the metrics file directory.

## Monitoring Metrics

cron-manager generates Prometheus-format metric files with the following dimensions:

| Dimension | Description | Values |
|-----------|-------------|--------|
| `failed` | Whether the job failed | 1 = failed, 0 = success |
| `delayed` | Whether the job timed out | 1 = timeout, 0 = normal |
| `duration` | Job execution duration (seconds) | Numeric value |
| `run` | Whether the job is running | 1 = running, 0 = finished |
| `last` | Last update timestamp | Unix timestamp |

### Metric Example

```prometheus
# TYPE cron_job gauge
cron_job{name="task_cron",dimension="failed"} 0
cron_job{name="task_cron",dimension="delayed"} 0
cron_job{name="task_cron",dimension="duration"} 10
cron_job{name="task_cron",dimension="run"} 0
cron_job{name="task_cron",dimension="last"} 1704067200
```

## License

This project is licensed under the [GNU General Public License v3.0](LICENSE).

## Acknowledgments

This project is based on the original work by [abohmeed/cronmanager](https://github.com/abohmeed/cronmanager). We extend our sincere gratitude to the original author and contributors for their excellent work.
