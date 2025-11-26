# radikron

![radikron](https://i.imgur.com/S0NIoIW.png)

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
  - [🖥️ Dual Interface Support](#dual-interface-support)
  - [🎯 Smart Rule-Based Matching](#smart-rule-based-matching)
  - [📁 Flexible File Organization](#flexible-file-organization)
  - [🛡️ Intelligent Download Management](#intelligent-download-management)
  - [➕ Manual Program Injection](#manual-program-injection)
  - [🌐 Multi-Region & Continuous Monitoring](#multi-region-continuous-monitoring)
  - [🖱️ GUI Features](#gui-features)
  - [🐳 Docker Support](#docker-support)
- [Requirements](#requirements)
- [Installation](#installation)
  - [CLI Version](#cli-version)
  - [GUI Version](#gui-version)
- [Configuration](#configuration)
  - [Configuration Options](#configuration-options)
  - [Rule Configuration](#rule-configuration)
  - [Example Configuration](#example-configuration)
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

### 🖥️ Dual Interface Support {#dual-interface-support}

- **CLI Version**: Lightweight command-line interface for servers and automation
- **GUI Version**: Modern graphical interface built with Wails v2 for desktop users
- **Shared Core**: Both versions use the same engine, ensuring consistent behavior

### 🎯 Smart Rule-Based Matching {#smart-rule-based-matching}

Flexible rules to automatically capture programs using multiple criteria: title/keyword matching, performer filtering, station selection, day-of-week and time-window filters.

### 📁 Flexible File Organization {#flexible-file-organization}

- Rule-based folder organization with configurable download directories
- Support for AAC (default) and MP3 formats
- Automatic ID3 tagging with program metadata

### 🛡️ Intelligent Download Management {#intelligent-download-management}

- Duplicate detection and file size validation
- Automatic retry with exponential backoff for failed downloads
- Concurrent downloads and automatic cleanup of stale failures

### ➕ Manual Program Injection {#manual-program-injection}

Manually add any program to the download queue (past or future), with persistent storage and automatic retry. GUI users can easily manage injections through the interface.

### 🌐 Multi-Region & Continuous Monitoring {#multi-region-continuous-monitoring}

- Area-based station filtering with support for extra stations from other regions
- Continuous background monitoring with scheduled fetching
- Graceful shutdown that waits for active downloads

### 🖱️ GUI Features {#gui-features}

Configuration management, station browser, program search with filtering, scheduled downloads view, monitoring control, and real-time activity logs.

![config-editor](https://github.com/user-attachments/assets/4f3e0509-d669-4e48-8adf-da50827925ed)
![program-search](https://github.com/user-attachments/assets/ba4741c9-37d8-4d20-ad8b-d6f2d21364bc)

### 🐳 Docker Support {#docker-support}

Pre-built Docker images with all dependencies included, ready for easy deployment.

## Requirements

### Common Requirements

- **[FFmpeg](https://ffmpeg.org/download.html)**: Required to combine m3u8 chunks to a single AAC file (or convert to MP3). Make sure `ffmpeg` exists in your `$PATH`.
  - The [docker image](#try-with-docker) already contains all the requirements including FFmpeg.

### GUI-Specific Requirements (for building from source)

If you're building the GUI version from source (Linux users), you'll also need:

- **Go 1.20+**
- **Node.js and pnpm**
- **Wails v2**: Install with `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

**Note**: macOS and Windows users can download pre-built binaries and don't need these build tools.

## Installation

### CLI Version

Install the command-line version:

```bash
go install github.com/iomz/radikron/cmd/radikron@latest
```

### GUI Version

#### macOS and Windows Users

Pre-built binaries are available for macOS and Windows. Download the latest release from the [Releases page](https://github.com/iomz/radikron/releases):

- **macOS**: Download the `.dmg` file for your architecture (Intel or Apple Silicon)
- **Windows**: Download the `.exe` file for your architecture (x64 or ARM64)

#### Linux Users

Linux users need to build from source. See the [GUI README](cmd/radikron-gui/README.md) for detailed setup instructions.

**Quick start for building on Linux**:

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

## Configuration

Create a configuration file (`config.yml`) to define rules for recording. The configuration supports various options to customize your download behavior:

### Configuration Options

- **`area-id`**: Your region code (e.g., `JP13` for Tokyo). If unset, defaults to your detected region.
- **`file-format`**: Output audio format - `aac` (default) or `mp3`.
- **`downloads`**: Directory path for downloaded files (default: `$HOME/Downloads/radiko` on all platforms).
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

**Important**: Rule order matters! When a program matches multiple rules, the first matching rule (in the order they appear in your config file) determines which folder the file is saved to. This allows you to prioritize certain rules by placing them earlier in your configuration.

### Example Configuration

```yaml
area-id: JP13 # if unset, default to "your" region
file-format: aac # audio format: aac or mp3, default is aac
downloads: ~/Downloads/radiko # download directory path, default is "$HOME/Downloads/radiko"
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

The base directory for downloads defaults to `$HOME/Downloads/radiko` (cross-platform: `~/Downloads/radiko` on Unix/macOS, `%USERPROFILE%\Downloads\radiko` on Windows). If the home directory cannot be determined, it falls back to `./radiko` in the current working directory.

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

By default, radikron will use `$HOME/Downloads/radiko` as the download directory. Temporary files are stored in the system temporary directory.

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
4. **Search programs**: Use the program search browser to find programs across all stations
5. **Manual injection**: Add programs manually to the download queue (useful for one-off downloads or programs that don't match your rules)
6. **View scheduled downloads**: See all scheduled downloads, including manual injections (marked with a "Manual" badge)
7. **Start monitoring**: Click "Start Monitoring" to begin automatic downloading
8. **Monitor activity**: Watch real-time updates in the activity log

**Manual Injection Workflow**:

- Search for programs using the program search browser
- Click on a program to view details
- Click "Inject" to add it to the download queue
- Past programs will download immediately; future programs will be scheduled
- View all scheduled downloads (including manual injections) in the Scheduled Downloads panel
- Delete manual injections if needed (they will be removed from the queue)

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
