# cron-manager

cron-manager is a Go tool for wrapping and monitoring cron jobs, publishing metrics through Prometheus Node Exporter's TextFile Collector.

## Features

- Execute cron job commands and monitor their status
- Measure job execution time with high precision (floating point seconds)
- Detect job failures and track exit codes
- Publish monitoring metrics via Prometheus TextFile Collector
- Support custom metrics file path
- Support job output logging
- Support idle wait mode (for Prometheus to detect job running status)
- Counter metrics for job execution statistics (success/failed counts)
- Prometheus best practices: separate metric names, HELP comments, proper label escaping

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

# Use custom metric name
cronmanager -n "job_cron" --metric my_cron_metric -- /usr/bin/command

# Disable metric writing
cronmanager -n "job_cron" --no-metric -- /usr/bin/command

# Custom idle wait duration (wait at least 120 seconds)
cronmanager -n "job_cron" -i 120 -- /usr/bin/command arg1 arg2

# Command with complex arguments (no need to escape)
cronmanager -n "update_cron" -- /usr/bin/php /var/www/app/console broadcast:entities:updated -e project -l 20000
```

### Command Line Options

| Short | Long | Description | Required | Default |
|-------|------|-------------|----------|---------|
| `-n` | `--name` | Job name (will appear in alerts) | ✅ | - |
| `-l` | `--log` | Log file path | ❌ | None (output will be discarded) |
| `-i` | `--idle` | Idle wait duration in seconds (ensures job runs for at least this duration for Prometheus detection) | ❌ | 0 (disabled) |
| `-d` | `--dir` | Directory for Prometheus exporter file | ❌ | `/var/lib/prometheus/node-exporter` or `COLLECTOR_TEXTFILE_PATH` env var |
| - | `--textfile` | Filename for Prometheus exporter file | ❌ | `crons.prom` |
| - | `--metric` | Metric name for Prometheus metrics | ❌ | `crontab` |
| - | `--no-metric` | Disable metric writing to Prometheus exporter file | ❌ | false |
| `-v` | `--version` | Display version information and exit | ❌ | - |
| - | `--` | Separator before command and its arguments | ✅ | - |

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
  --collector.textfile.directory=/var/lib/prometheus/node-exporter
```

### Custom Metrics File Path

You can customize both the directory and filename for the Prometheus exporter file:

**Directory** (priority order):
1. **Command line argument `--dir` or `-d`** (highest priority):
```bash
cronmanager --name job_cron --dir /custom/path/to/directory -- command
```

2. **Environment variable `COLLECTOR_TEXTFILE_PATH`**:
```bash
export COLLECTOR_TEXTFILE_PATH=/custom/path/to/directory
cronmanager --name job_cron -- command
```

3. **Default path** (lowest priority): `/var/lib/prometheus/node-exporter`

**Filename**:
- Use `--textfile` to specify a custom filename (default: `crons.prom`):
```bash
cronmanager --name job_cron --textfile my-metrics.prom -- command
```

**Combined example**:
```bash
cronmanager --name job_cron --dir /tmp/prometheus --textfile custom.prom -- command
```

**Metric name customization**:
- Use `--metric` to specify a custom metric name (default: `crontab`):
```bash
cronmanager --name job_cron --metric my_cron_metric -- command
```

**Disable metric writing**:
- Use `--no-metric` to disable metric writing entirely:
```bash
cronmanager --name job_cron --no-metric -- command
```

### Permissions

Ensure the user running cron-manager has write permissions to the metrics file directory.

## Monitoring Metrics

cron-manager generates Prometheus-format metric files following best practices with separate metric names and proper types.

### Gauge Metrics

Gauge metrics represent the current state of a job:

| Metric Name | Type | Description | Values |
|-------------|------|-------------|--------|
| `{prefix}_failed` | gauge | Whether the job failed | 1 = failed, 0 = success |
| `{prefix}_exit_code` | gauge | Exit code of the last job execution | Numeric exit code (0 = success) |
| `{prefix}_duration_seconds` | gauge | Duration of the last job execution | Floating point seconds (e.g., 10.25) |
| `{prefix}_running` | gauge | Whether the job is currently running | 1 = running, 0 = finished |
| `{prefix}_last_run_timestamp_seconds` | gauge | Timestamp of the last job execution | Unix timestamp |

Where `{prefix}` is the metric name prefix (default: `crontab`, customizable via `--metric` flag).

### Counter Metrics

Counter metrics track cumulative statistics:

| Metric Name | Type | Description |
|-------------|------|-------------|
| `{prefix}_runs_total` | counter | Total number of job runs with status label |

The `runs_total` counter includes a `status` label with values:
- `status="started"` - Job execution started
- `status="success"` - Job completed successfully
- `status="failed"` - Job failed with non-zero exit code

### Metric Example

```prometheus
# HELP crontab_failed Whether the job failed (1 = failed, 0 = success)
# TYPE crontab_failed gauge
crontab_failed{name="task_cron"} 0

# HELP crontab_exit_code Exit code of the last job execution
# TYPE crontab_exit_code gauge
crontab_exit_code{name="task_cron"} 0

# HELP crontab_duration_seconds Duration of the last job execution in seconds
# TYPE crontab_duration_seconds gauge
crontab_duration_seconds{name="task_cron"} 10.25

# HELP crontab_running Whether the job is currently running (1 = running, 0 = finished)
# TYPE crontab_running gauge
crontab_running{name="task_cron"} 0

# HELP crontab_last_run_timestamp_seconds Timestamp of the last job execution
# TYPE crontab_last_run_timestamp_seconds gauge
crontab_last_run_timestamp_seconds{name="task_cron"} 1704067200

# HELP crontab_runs_total Total number of job runs
# TYPE crontab_runs_total counter
crontab_runs_total{name="task_cron",status="success"} 100
crontab_runs_total{name="task_cron",status="failed"} 5
crontab_runs_total{name="task_cron",status="started"} 105
```

### Querying Metrics

With the new metric design, queries are more intuitive:

```promql
# Check if any job is currently running
crontab_running == 1

# Get success rate
rate(crontab_runs_total{status="success"}[5m]) / rate(crontab_runs_total[5m])

# Get average duration
avg(crontab_duration_seconds)

# Alert on job failures
crontab_failed == 1

# Get jobs with non-zero exit codes
crontab_exit_code != 0
```

## License

This project is licensed under the [GNU General Public License v3.0](LICENSE).

## Grafana Dashboard

A pre-configured Grafana dashboard is available in `grafana-dashboard.json`. This dashboard provides comprehensive visualization of cron job metrics.

### Dashboard Features

The dashboard includes the following panels:

1. **Job Status (Last Run)** - Shows success/failure status of the last execution
2. **Currently Running** - Indicates which jobs are currently executing
3. **Job Duration** - Time series graph showing execution duration over time
4. **Job Execution Rate** - Shows the rate of successful and failed executions per second
5. **Success Rate** - Gauge showing the percentage of successful executions in the last 5 minutes
6. **Exit Code** - Displays the exit code from the last execution
7. **Last Run Time** - Shows how long ago each job last ran
8. **Jobs Overview Table** - Comprehensive table with all metrics for quick overview

### Dashboard Variables

- **Datasource**: Select your Prometheus datasource
- **Job Name**: Filter by specific job names (supports multi-select and "All")

### Importing the Dashboard

1. Open Grafana web interface
2. Navigate to **Dashboards** → **Import**
3. Upload the `grafana-dashboard.json` file or paste its contents
4. Select your Prometheus datasource
5. Click **Import**

### Dashboard Requirements

- Grafana version 8.0 or higher
- Prometheus datasource configured in Grafana
- Prometheus Node Exporter collecting metrics from cron-manager

## Acknowledgments

This project is based on the original work by [abohmeed/cronmanager](https://github.com/abohmeed/cronmanager). We extend our sincere gratitude to the original author and contributors for their excellent work.
