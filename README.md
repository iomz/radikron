# radikron

![radikron](https://i.imgur.com/Nnby5YQ.png)

[![build status](https://github.com/iomz/radikron/workflows/build/badge.svg)](https://github.com/iomz/radikron/actions?query=workflow%3Abuild)
[![docker status](https://github.com/iomz/radikron/actions/workflows/docker.yml/badge.svg)](https://github.com/iomz/radikron/actions/workflows/docker.yml)

[![docker image size](https://ghcr-badge.egpl.dev/iomz/radikron/size)](https://github.com/iomz/radikron/pkgs/container/radikron)
[![godoc](https://godoc.org/github.com/iomz/radikron?status.svg)](https://godoc.org/github.com/iomz/radikron)
[![codecov](https://codecov.io/gh/iomz/radikron/branch/dev/graph/badge.svg?token=fjhUp7BLPB)](https://codecov.io/gh/iomz/radikron)
[![go report](https://goreportcard.com/badge/github.com/iomz/radikron)](https://goreportcard.com/report/github.com/iomz/radikron)
[![license: GPL v3](https://img.shields.io/badge/license-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)

Sometimes we miss our favorite shows on [radiko](https://radiko.jp/) and they get vanished from <http://radiko.jp/#!/timeshift> – let's just keep them automatically saved locally, from AoE.

**Disclaimer**:

- Never use this program for commercial purposes.

---

<!-- vim-markdown-toc GFM -->

- [Requirements](#requirements)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
  - [Try with Docker](#try-with-docker)
- [Build the image yourself](#build-the-image-yourself)
- [Credit](#credit)

<!-- vim-markdown-toc -->

## Requirements

radikron requires [FFmpeg](https://ffmpeg.org/download.html) to combine m3u8 chunks to a single aac file (or convert to mp3).

Make sure `ffmpeg` exists in your `$PATH`.

The [docker image](#try-with-docker) already contains all the requirements including ffmpeg.

## Installation

```bash
go install github.com/iomz/radikron/cmd/radikron@latest
```

## Configuration

Create a configuration file (`config.yml`) to define rules for recording:

```yaml
area-id: JP13 # if unset, default to "your" region
file-format: aac # audio format: aac or mp3, default is aac
downloads: downloads # download directory name, default is "downloads"
extra-stations:
  - ALPHA-STATION # include stations not in your region
ignore-stations:
  - JOAK # ignore stations from search
minimum-output-size: 2 # do not save an audio below this size (in MB), default is 1 (MB)
rules:
  airship: # name your rule as you like
    folder: citypop # (optional) organize downloads into subfolders
    station-id: FMT # (optional) the station_id, if not available by default, automatically add this station to the watch list
    title: "GOODYEAR MUSIC AIRSHIP～シティポップ レイディオ～" # this can be a partial match
  citypop:
    keyword: "シティポップ" # search by keyword (also a partial match)
    window: 48h # only within the past window from the current time
  hiccorohee:
    pfm: "ヒコロヒー" # search by pfm
  trad:
    dow: # filter by day of the week (e.g, Mon, tue, WED)
      - wed
      - thu
    station-id: FMT
    title: "THE TRAD"
```

In addition, set `${RADICRON_HOME}` to set the base directory for downloads and temporary files. The actual download location will be `${RADICRON_HOME}/{downloads}` (or the value specified in the `downloads` config option).

## Usage

```bash
mkdir -p ./radiko/{downloads,tmp} && RADICRON_HOME=./radiko radikron -c config.yml
```

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

This project is heavily based on [yyoshiki41/go-radiko](https://github.com/yyoshiki41/go-radiko) and [yyoshiki41/radigo](https://github.com/yyoshiki41/radigo), and therefore follows the [GPLv3 License](https://github.com/yyoshiki41/radigo/blob/main/LICENSE).
