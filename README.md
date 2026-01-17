# CronManager

[![Go Version](https://img.shields.io/badge/go-1.16+-00ADD8?style=flat-square&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/license-GPL--3.0-green.svg?style=flat-square)](LICENSE)
[![CI](https://img.shields.io/github/workflow/status/abohmeed/cronmanager/CI?style=flat-square)](https://github.com/abohmeed/cronmanager/actions)

CronManager is a tool written in Go that wraps and monitors cron jobs. It publishes monitoring metrics through Prometheus Node Exporter's TextFile collector.

## Table of Contents

- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Usage](#usage)
- [Command Line Options](#command-line-options)
- [Monitoring and Alerting](#monitoring-and-alerting)
- [License](#license)

## Features

- ✅ Execute cron job commands
- ✅ Measure command execution time
- ✅ Check command exit status
- ✅ Publish monitoring metrics through Prometheus Node Exporter's TextFile collector
- ✅ Support custom textfile collector path
- ✅ Support log file output

## Requirements

For the tool to work correctly, you need to have **Prometheus Node Exporter** installed on the machine with the textfile collector enabled and the directory specified.

### Node Exporter Configuration Example

```bash
/opt/prometheus/exporters/node_exporter_current/node_exporter \
  --collector.conntrack \
  --collector.diskstats \
  --collector.entropy \
  --collector.filefd \
  --collector.filesystem \
  --collector.loadavg \
  --collector.mdadm \
  --collector.meminfo \
  --collector.netdev \
  --collector.netstat \
  --collector.stat \
  --collector.time \
  --collector.vmstat \
  --web.listen-address=0.0.0.0:9100 \
  --log.level=info \
  --collector.textfile \
  --collector.textfile.directory=/opt/prometheus/exporters/dist/textfile
```

### Custom Textfile Collector Path

You can use the environment variable `COLLECTOR_TEXTFILE_PATH` to specify a custom textfile collector path:

```bash
export COLLECTOR_TEXTFILE_PATH=/custom/path/to/textfile
/opt/prometheus/exporters/node_exporter_current/node_exporter \
  --collector.textfile \
  --collector.textfile.directory=$COLLECTOR_TEXTFILE_PATH \
  # ... other parameters
```

## Installation

### Build Binary

```bash
env GOOS=linux go build -o cronmanager
```

### Install to System Path

```bash
sudo mv cronmanager /usr/local/bin/
```

## Usage

Basic usage:

```bash
cronmanager -c command -n jobname [ -t time in seconds ] [ -l log file ]
```

### Important Notes

**Limitations of the `command` parameter**: You cannot use bash shell or its built-in commands as the command. The following examples will **not work**:

```bash
# ❌ These commands will not work
cronmanager -c "echo 'hello' > somefile"
cronmanager -c "command1; command2; command3"
```

**Correct usage**: The command should be a binary file with optional arguments. The following are **valid** command examples:

```bash
# ✅ These commands will work
cronmanager -c "/usr/bin/php /var/www/webdir/console broadcast:entities:updated -e project -l 20000" -n update_entities_cron
cronmanager -c "/usr/bin/python3 /path/to/python_script.py" -n python_script_cron
```

### Idle Wait Parameter

The `-i` parameter adds a wait time at the end of the process to let Prometheus detect that the job is running. It will wait for 60 seconds minus the time that has already elapsed. For example, if the command takes 20 seconds to finish, it will wait an additional 40 seconds at the end.

## Command Line Options

| Option | Description | Required | Default |
|--------|-------------|----------|---------|
| `-c` | The command to execute | ✅ Yes | - |
| `-n` | Job name (will appear in alerts) | ✅ Yes | "Generic" |
| `-t` | Job timeout in seconds, exceeding this time will trigger an alert | ❌ No | 3600 (1 hour) |
| `-l` | Log file path to store the cron job output | ❌ No | None (output will be discarded) |
| `-i` | Enable idle wait to let Prometheus detect the job is running | ❌ No | false |
| `-version` | Display version information and exit | ❌ No | - |

**Note**: If you don't specify the `-n` parameter, the command will default to "Generic" as the job name. It's recommended to append `_cron` suffix to the job name for easier distinction when viewing alerts in Prometheus or Grafana.

## Monitoring and Alerting

### File Path Requirements

For the tool to work correctly, ensure that the textfile collector path exists and the user running cronmanager has write permissions to that path.

Default path: `/opt/prometheus/exporters/dist/textfile/`

If using a custom path (via the `COLLECTOR_TEXTFILE_PATH` environment variable), ensure that path exists and has write permissions.

### Metrics File Format

When cronmanager starts a job, it creates a metrics file in the specified path. The filename consists of the job name followed by the `.prom` extension.

For example, running the following command:

```bash
cronmanager -c "some_command some_arguments" -n "myjob"
```

Will create the file: `/opt/prometheus/exporters/dist/textfile/myjob.prom` (or the corresponding file in the custom path)

Example file contents:

```prometheus
# TYPE cron_job gauge
cron_job{name="cron1",dimension="failed"} 0
cron_job{name="cron1",dimension="delayed"} 0
cron_job{name="cron1",dimension="duration"} 10
cron_job{name="cron1",dimension="run"} 1
cron_job{name="cron1",dimension="last"} 1234567890
```

These values change according to the job status:
- `failed`: 1 if the job failed, 0 otherwise
- `delayed`: 1 if the job timed out, 0 otherwise
- `duration`: Job execution time in seconds
- `run`: 1 if the job is running, 0 otherwise
- `last`: Last update timestamp (Unix timestamp)

## License

This project is licensed under the [GNU General Public License v3.0](LICENSE).
