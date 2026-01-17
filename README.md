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
cronmanager -c "command" -n "job_name" [options]
```

### Examples

```bash
# Execute PHP script
cronmanager -c "/usr/bin/php /var/www/app/console task:run" -n "task_cron"

# Execute Python script with logging
cronmanager -c "/usr/bin/python3 /path/to/script.py" -n "script_cron" -l /var/log/cron.log

# Enable idle wait mode
cronmanager -c "/usr/bin/command" -n "job_cron" -i
```

### Command Line Options

| Option | Description | Required | Default |
|--------|-------------|----------|---------|
| `-c` | Command to execute (executable path with arguments) | ✅ | - |
| `-n` | Job name (will appear in alerts) | ✅ | "Generic" |
| `-l` | Log file path | ❌ | None (output will be discarded) |
| `-i` | Enable idle wait (wait up to 60 seconds for Prometheus detection) | ❌ | false |
| `-version` | Display version information and exit | ❌ | - |

### Notes

- The `-c` parameter must be an executable file path; shell built-ins or pipe operations are not supported
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

Specify a custom path using the `COLLECTOR_TEXTFILE_PATH` environment variable:

```bash
export COLLECTOR_TEXTFILE_PATH=/custom/path/to/textfile
```

Default path: `/var/cache/prometheus/crons.prom`

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
