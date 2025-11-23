# radikron

![radikron](https://i.imgur.com/BiTAPQz.png)

[![build status](https://github.com/iomz/radikron/workflows/build/badge.svg)](https://github.com/iomz/radikron/actions?query=workflow%3Abuild)
[![docker status](https://github.com/iomz/radikron/actions/workflows/docker.yml/badge.svg)](https://github.com/iomz/radikron/actions/workflows/docker.yml)

[![docker image size](https://ghcr-badge.egpl.dev/iomz/radikron/size)](https://github.com/iomz/radikron/pkgs/container/radikron)
[![godoc](https://godoc.org/github.com/iomz/radikron?status.svg)](https://godoc.org/github.com/iomz/radikron)
[![codecov](https://codecov.io/gh/iomz/radikron/branch/dev/graph/badge.svg?token=fjhUp7BLPB)](https://codecov.io/gh/iomz/radikron)
[![go report](https://goreportcard.com/badge/github.com/iomz/radikron)](https://goreportcard.com/report/github.com/iomz/radikron)
[![license: GPL v3](https://img.shields.io/badge/license-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)

Sometimes we miss our favorite shows on [radiko.jp](https://radiko.jp/) and they get vanished from <http://radiko.jp/#!/timeshift> – let's just keep them automatically saved in your local disk, from AoE.

**Disclaimer**:

- Never use this program for commercial purposes.

---

<!-- vim-markdown-toc GFM -->

- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
  - [CLI Version](#cli-version)
  - [GUI Version](#gui-version)
- [Configuration](#configuration)
  - [ID3 Tags](#id3-tags)
- [Usage](#usage)
  - [CLI Usage](#cli-usage)
  - [GUI Usage](#gui-usage)
  - [Try with Docker](#try-with-docker)
- [Build the image yourself](#build-the-image-yourself)
- [Credit](#credit)

<!-- vim-markdown-toc -->

## Features

radikron is a powerful, automated radio program downloader for [radiko](https://radiko.jp/) available in both **CLI** and **GUI** versions. Both versions share the same core logic, ensuring consistent behavior and reliability.

### 🖥️ Dual Interface Support

- **CLI Version**: Lightweight command-line interface perfect for servers and automation
- **GUI Version**: Modern graphical interface built with Wails v2 for desktop users
- **Shared Core**: Both versions use the same underlying engine, so bug fixes and features benefit both interfaces

### 🎯 Smart Rule-Based Matching

Create flexible rules to automatically capture your favorite programs using multiple matching criteria:

- **Title Matching**: Match programs by title (supports partial matching)
- **Keyword Search**: Find programs containing specific keywords in title or description
- **Personality/Performer Matching**: Filter by program host or performer (`pfm`)
- **Station Filtering**: Target specific radio stations or include stations from other regions
- **Day of Week Filtering**: Download programs only on specific days (e.g., every Wednesday and Thursday)
- **Time Window Filtering**: Only download programs within a specified time window (e.g., last 48 hours)

### 📁 Flexible File Organization

- **Custom Download Directories**: Configure where your files are saved
- **Rule-Based Folders**: Automatically organize downloads into subfolders based on matching rules
- **Configurable File Formats**: Choose between AAC (default) or MP3 output formats

### 🏷️ Automatic ID3 Tagging

All downloaded files are automatically tagged with rich metadata:

- Program title, artist, album, and year
- Program information and comments
- Rule name as Album Artist for easy organization
- Works with both AAC and MP3 formats

### 🛡️ Intelligent Download Management

- **Duplicate Detection**: Automatically skips files that already exist (checks both default and rule-specific folders)
- **Minimum File Size Validation**: Rejects corrupted or incomplete downloads below a specified size
- **Automatic Retry**: Built-in retry mechanism for failed downloads
- **Concurrent Downloads**: Downloads multiple programs simultaneously for efficiency

### 🌐 Multi-Region Support

- **Area-Based Filtering**: Automatically filters stations based on your region
- **Extra Stations**: Include stations from other regions not available in your area
- **Station Blacklist**: Ignore specific stations you don't want to monitor

### 🔄 Continuous Monitoring

- **Scheduled Fetching**: Automatically checks for new programs at optimal intervals
- **Background Operation**: Runs continuously, monitoring and downloading programs as they become available
- **Graceful Shutdown**: Waits for downloads to complete before exiting

### 🖱️ GUI Features

The GUI version provides a user-friendly interface with:

- **Configuration Management**: Load and manage configuration files through an intuitive interface
- **Station Browser**: View all available radio stations in your region
- **Monitoring Control**: Start and stop automatic monitoring with a single click
- **Real-Time Activity Log**: Monitor download progress, completions, and errors in real-time
- **Event System**: Real-time updates via Wails events for instant feedback

### 🐳 Docker Support

- Pre-built Docker images for easy deployment
- No need to install FFmpeg or other dependencies manually
- Ready-to-use Docker Compose configuration

## Requirements

radikron requires [FFmpeg](https://ffmpeg.org/download.html) to combine m3u8 chunks to a single aac file (or convert to mp3).

Make sure `ffmpeg` exists in your `$PATH`.

The [docker image](#try-with-docker) already contains all the requirements including ffmpeg.

## Installation

### CLI Version

Install the command-line version:

```bash
go install github.com/iomz/radikron/cmd/radikron@latest
```

### GUI Version

The GUI version requires building from source. See the [GUI README](cmd/radikron-gui/README.md) for detailed setup instructions.

**Quick start for GUI development**:

```bash
# Install frontend dependencies
cd cmd/radikron-gui/frontend
pnpm install

# Return to GUI directory
cd ..

# Run in development mode
wails dev

# Or build for production
wails build
```

**Prerequisites for GUI**:

- Go 1.20+
- Node.js and pnpm
- Wails v2 (install with `go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

## Configuration

Create a configuration file (`config.yml`) to define rules for recording. The configuration supports various options to customize your download behavior:

### Configuration Options

- **`area-id`**: Your region code (e.g., `JP13` for Tokyo). If unset, defaults to your detected region.
- **`file-format`**: Output audio format - `aac` (default) or `mp3`.
- **`downloads`**: Directory name for downloaded files (default: `downloads`). Combined with `${RADICRON_HOME}` to form the full path.
- **`extra-stations`**: List of station IDs to include even if they're not in your region.
- **`ignore-stations`**: List of station IDs to exclude from monitoring.
- **`minimum-output-size`**: Minimum file size in MB (default: 1 MB). Files smaller than this are rejected as potentially corrupted.
- **`max-downloading-concurrency`**: Maximum number of concurrent download operations (default: 64). Only included in config if different from default.
- **`max-encoding-concurrency`**: Maximum number of concurrent MP3 encoding operations (default: 2). Set lower than downloading concurrency since encoding is CPU-intensive. Only included in config if different from default.

### Rule Configuration

Each rule can use one or more of the following matching criteria (all support partial matching):

- **`title`**: Match programs by title
- **`keyword`**: Match programs containing the keyword in title or description
- **`pfm`**: Match programs by personality/performer name
- **`station-id`**: Filter by specific station (also adds the station to watch list if not in your region)
- **`dow`**: Filter by day of week (e.g., `mon`, `tue`, `wed`, `thu`, `fri`, `sat`, `sun`)
- **`window`**: Time window filter (e.g., `48h` for last 48 hours, `7d` for last 7 days)
- **`folder`**: (Optional) Organize downloads for this rule into a subfolder

Rules are evaluated with AND logic - a program must match all specified criteria in a rule.

### Example Configuration

```yaml
area-id: JP13 # if unset, default to "your" region
file-format: aac # audio format: aac or mp3, default is aac
downloads: downloads # download directory name, default is "downloads"
extra-stations:
  - ALPHA-STATION # include stations not in your region
ignore-stations:
  - JOAK # ignore stations from search
minimum-output-size: 2 # do not save an audio below this size (in MB), default is 1 (MB)
# max-downloading-concurrency: 64  # Maximum concurrent download operations (default: 64)
# max-encoding-concurrency: 2  # Maximum concurrent encoding operations for MP3 conversion (default: 2)
rules:
  midday: # name your rule as you like
    folder: "MIDDAY LOUNGE" # (optional) organize downloads into subfolders
    station-id: FMJ # (optional) the station_id, if not available by default, automatically add this station to the watch list
    title: "MIDDAY LOUNGE" # this can be a partial match
  citypop:
    keyword: "シティポップ" # search by keyword (also a partial match)
    window: 48h # only within the past window from the current time
  hiccorohee:
    pfm: "ヒコロヒー" # search by pfm (i.e., the DJ/MC)
  trad:
    dow: # filter by day of the week (e.g, Mon, tue, WED)
      - wed
      - thu
    station-id: FMT
    title: "THE TRAD"
```

The base directory for downloads and temporary files is determined by the `RADICRON_HOME` environment variable. If not set, it defaults to `./radiko` in the current working directory. The actual download location will be `${RADICRON_HOME}/{downloads}` (or the value specified in the `downloads` config option).

### ID3 Tags

All downloaded audio files (both AAC and MP3) are automatically tagged with ID3v2 metadata:

- **Title**: File base name (format: `YYYY-MM-DD-HHMM_StationID_ProgramTitle`)
- **Artist**: Program personality/performer (`pfm`)
- **Album**: Program title
- **Year**: Program start year
- **Comment**: Program information (`info`)
- **Album Artist**: Rule name (if the program matched a rule)

These tags are embedded in both AAC and MP3 files, making it easy to organize and identify your downloaded programs in music players and media libraries.

## Usage

### CLI Usage

**Basic Usage**:

Simply run radikron with your configuration file:

```bash
radikron -c config.yml
```

By default, radikron will use `./radiko` as the base directory (containing `downloads` and `tmp` subdirectories). To use a different location, set the `RADICRON_HOME` environment variable:

```bash
RADICRON_HOME=/path/to/your/directory radikron -c config.yml
```

**Note**: radikron automatically creates all necessary directories (download directories, subfolders, and temporary directories) when needed. You don't need to create them manually.

The application will:

- Connect to radiko and authenticate
- Fetch program schedules for all monitored stations
- Match programs against your configured rules
- Download matching programs automatically
- Tag files with ID3 metadata
- Continue monitoring and downloading on a schedule

**Command-Line Options**:

- **`-c <file>`**: Specify the configuration file (default: `config.yml`)
- **`-d`**: Enable debug mode with detailed logging
- **`-v`**: Print version information

**Running as a Service**:

radikron is designed to run continuously. It automatically:

- Schedules the next fetch time based on program availability
- Waits for downloads to complete before checking again
- Handles interruptions gracefully (waits for in-progress downloads on shutdown)

For production use, consider running it as a systemd service or using a process manager like `supervisord`.

### GUI Usage

The GUI version provides a visual interface for managing radikron:

1. **Launch the application**: Run the built binary or use `wails dev` for development
2. **Load configuration**: Use the configuration panel to load your `config.yml` file
3. **View stations**: Browse available radio stations in your region
4. **Start monitoring**: Click "Start Monitoring" to begin automatic downloading
5. **Monitor activity**: Watch real-time updates in the activity log

The GUI shares the same configuration format and behavior as the CLI version, so you can use the same `config.yml` file with both interfaces.

For detailed GUI setup and development instructions, see the [GUI README](cmd/radikron-gui/README.md).

### Try with Docker

By default, it mounts `./config.yml` and `./radiko` to the container.

```console
docker compose up
```

## Build the image yourself

In case the [image](https://github.com/iomz/radikron/pkgs/container/radikron) is not available for your platform:

```console
docker compose build
```

## Credit

This project started off from [yyoshiki41/go-radiko](https://github.com/yyoshiki41/go-radiko) and [yyoshiki41/radigo](https://github.com/yyoshiki41/radigo), and therefore follows the [GPLv3 License](https://github.com/yyoshiki41/radigo/blob/main/LICENSE).
