# Radikron GUI

This is the graphical user interface for Radikron, built with [Wails v3](https://v3.wails.io) (beta).

## Overview

The GUI provides a user-friendly interface for managing Radikron while keeping the CLI version fully functional. Both versions share the same core logic from the `radikron` package.

## Features

- **Configuration Management**: Load and manage configuration files
- **Station Browser**: View available radio stations
- **Monitoring Control**: Start/stop the automatic monitoring and downloading
- **Activity Log**: Real-time view of download activities
- **Event System**: Real-time updates via Wails events

## Development

### Prerequisites

- Go 1.26+
- Node.js 20+
- pnpm 10+
- Wails v3 CLI `v3.0.0-beta.28`: `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.28`
- On Linux: GTK4 and WebKitGTK 6.0 development packages (`sudo apt install build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev` on Ubuntu)

### Setup

1. Install frontend dependencies:

   ```bash
   cd frontend
   pnpm install
   ```

2. Return to the GUI directory:

   ```bash
   cd ..
   ```

3. Run in development mode:

   ```bash
   wails3 dev
   ```

   This will:

   - Start the Wails dev server
   - Open a native application window
   - Enable hot-reload for frontend changes

4. Test the development build:

   The application window will open automatically. You can test the GUI functionality by interacting with the interface. Note that:

   - **Frontend changes** (React/TypeScript/CSS) will reload automatically in the window
   - **Go code changes** rebuild and restart the application automatically

### Building

Build the application:

```bash
wails3 build
```

This creates the binary in `bin/`.
On macOS, `wails3 task package` also assembles `bin/Radikron.app`.
Pass `ARCH=amd64` or `ARCH=arm64` to `wails3 task build` or `wails3 task package` to build for another architecture; the `wails3 build` and `wails3 package` shortcuts do not forward it.

The frontend bindings in `frontend/bindings/` are committed so frontend tests run without Go.
Regenerate them after changing exported `App` methods or their types:

```bash
wails3 task common:generate:bindings
```

### Testing

Run backend, frontend, type, build, and lint checks from repository root:

```bash
go test ./...
pnpm --dir cmd/radikron-gui/frontend test
pnpm --dir cmd/radikron-gui/frontend run typecheck
pnpm --dir cmd/radikron-gui/frontend run build
(cd cmd/radikron-gui && wails3 build)
golangci-lint run ./...
git diff --check
```

GUI backend tests use deterministic event and monitoring seams; they do not launch native Wails window or contact download services.

## Architecture

### Structure

```text
cmd/radikron-gui/
├── main.go          # Wails entry point
├── app.go           # App struct with exposed methods
├── Taskfile.yml     # Wails v3 build tasks (desktop parts of the wails3 template)
├── build/           # Build configuration, Info.plist, Windows manifest, app icon
└── frontend/
    ├── index.html   # HTML entry point
    ├── bindings/    # Generated TypeScript bindings for App and its models
    ├── src/
    │   ├── main.tsx # React entry point
    │   ├── App.tsx  # Main React component
    │   └── style.css
    ├── tsconfig.json # TypeScript configuration
    ├── vite.config.ts # Vite configuration with React plugin
    └── package.json
```

### Key Components

- **App struct**: Wraps radikron functionality and exposes methods to frontend
- **Event System**: Uses Wails events for real-time updates
- **Monitoring Loop**: Runs in a goroutine, similar to CLI version

### Exposed Methods

- `GetConfig()` - Get current configuration
- `LoadConfig(filename)` - Load configuration from file
- `GetAvailableStations()` - Get list of available stations
- `GetMonitoringStatus()` - Check if monitoring is active
- `StartMonitoring()` - Start the monitoring loop
- `StopMonitoring()` - Stop the monitoring loop
- `GetStatus()` - Get the automatic download status: monitoring, last and next check, and configuration or station loading errors
- `Retry()` - Retry what failed: startup, the last configuration reload, or the station list
- `GetDownloads()` - Get scheduled programs with their download state (waiting, available, downloading, failed, expired) and the downloads finished in this session
- `UpdatesSupported()` - Report whether this build can update itself (macOS `.app` bundles and Windows)
- `AvailableUpdate()` - Get the newest release version found by the background update check
- `CheckForUpdates()` - Open the Wails update window

### Events

- `monitoring-started` - Emitted when monitoring starts
- `monitoring-stopped` - Emitted when monitoring stops
- `download-started` - Emitted when a download begins
- `download-completed` - Emitted when a download completes
- `download-failed` - Emitted when a download fails
- `config-loaded` - Emitted when configuration is loaded
- `status-changed` - Emitted with the new status whenever monitoring, check times, or loading errors change
- `downloads-changed` - Emitted when a download state or the schedule changes; reload `GetDownloads()`
- `update-available` - Emitted when the background update check finds a newer release

### Updates

`selfupdate.go` configures the Wails v3 updater (`app.Updater`) against GitHub Releases.
The Wails GitHub provider only reads checksum files, so `signedProvider` wraps it and attaches each asset's `.sig` as an `ecdsa-p256` signature, which the updater verifies against `internal/releasekey/public_key.pem` before installing.
The running version is `info.version` from `build/config.yml`, embedded at build time.

## Integration with Core Package

The GUI uses the same core packages as the CLI:

- `github.com/iomz/radikron` - Core download and asset logic
- `github.com/iomz/radikron/internal/config` - Configuration management

This ensures:

- ✅ No code duplication
- ✅ Bug fixes benefit both interfaces
- ✅ Consistent behavior between CLI and GUI

## Troubleshooting & Development Tips

### Common Issues and Quick Fixes

#### Wails Build Errors

**Issue**: `wails3 build` or `wails3 dev` fails with errors about missing dependencies or build tools.

**Solutions**:

- Ensure Go 1.26+ is installed: `go version`
- Verify Wails is properly installed: `wails3 version`
- Check system dependencies: `wails3 doctor`
- Reinstall Wails if needed: `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.28`
- Check that `GOPATH` and `GOROOT` are set correctly: `go env GOPATH GOROOT`
- Clean and rebuild: `go clean -cache && go mod tidy`

#### Frontend Dev Server Not Starting

**Issue**: Frontend changes don't hot-reload or dev server fails to start.

**Solutions**:

- Ensure you're in the correct directory: `cd cmd/radikron-gui`
- Install/update frontend dependencies: `cd frontend && pnpm install`
- Check for port conflicts (`wails3 dev` uses Vite port 9245; override with `WAILS_VITE_PORT`)
- Verify Node.js version: `node --version` (should be compatible with package.json)
- Clear Vite cache: `rm -rf frontend/node_modules/.vite`

#### TypeScript/Vite Config Issues

**Issue**: TypeScript errors, module resolution failures, or Vite build errors.

**Solutions**:

- Check TypeScript configuration: [`frontend/tsconfig.json`](frontend/tsconfig.json)
- Verify Vite configuration: [`frontend/vite.config.ts`](frontend/vite.config.ts)
- Ensure all dependencies are installed: `cd frontend && pnpm install`
- Check for TypeScript version conflicts: `cd frontend && pnpm list typescript`
- Restart the TypeScript server in your IDE

#### Missing Go Environment or Modules

**Issue**: Go build fails with "cannot find package" or module errors.

**Solutions**:

- Verify Go environment: `go env`
- Download dependencies: `go mod download`
- Tidy modules: `go mod tidy`
- Verify module path in `go.mod` matches your repository structure
- Check that you're running commands from the repository root

#### File Write Permissions

**Issue**: Application fails to write configuration files or download files.

**Solutions**:

- Check file permissions on the target directory
- On macOS/Linux: `chmod -R u+w /path/to/directory`
- Verify the application has write access to the config file location
- Check that the download directory exists and is writable
- Review configuration file path in [`config.yml`](../../config.yml)

#### Common Runtime Errors

**Issue**: Application crashes or behaves unexpectedly at runtime.

**Solutions**:

- Check application logs (see Debugging Tips below)
- Verify configuration file format is valid YAML
- Ensure required configuration keys are present
- Check network connectivity for station data and downloads
- Verify radiko authentication tokens are valid

### Local Development Commands

#### Start Frontend Dev Server (Standalone)

To run the frontend independently for UI development:

```bash
cd frontend
pnpm run dev
```

This starts Vite dev server on `http://localhost:5173` (or next available port).

#### Build Steps

**Full development workflow**:

```bash
# 1. Install frontend dependencies
cd cmd/radikron-gui/frontend
pnpm install

# 2. Return to GUI directory
cd ..

# 3. Run in development mode (includes hot-reload)
wails3 dev
```

**Production build**:

```bash
# From cmd/radikron-gui directory
wails3 build

# Output will be in bin/
```

**Frontend-only build** (for testing):

```bash
cd frontend
pnpm run build
```

### Debugging Tips

#### Enable Verbose Logging

**Wails dev logging**: raise `dev_mode.log_level` in [`build/config.yml`](build/config.yml) (for example to `debug`), then run:

```bash
wails3 dev
```

**Go debug logging**: Check [`main.go`](main.go) and [`app.go`](app.go) for log statements. You can add more detailed logging using the logger instance.

**Frontend console**: Open browser devtools in the Wails window (see below) to see console logs.

#### Check Wails DevTools

1. While running `wails3 dev`, right-click in the application window
2. Select "Inspect" or "Inspect Element" to open DevTools
3. Use the Console tab for JavaScript errors and logs
4. Use the Network tab to inspect API calls
5. Use the React DevTools extension for component debugging

#### Inspect Network Requests

- Open Wails DevTools (right-click → Inspect)
- Navigate to Network tab
- Filter by XHR/Fetch to see API calls
- Check request/response payloads and status codes
- Verify CORS headers if making external requests

#### Additional Debugging

- **Check Wails configuration**: Review [`build/config.yml`](build/config.yml) and [`Taskfile.yml`](Taskfile.yml) for build settings
- **Verify entry points**: Check [`main.go`](main.go) for application initialization
- **Frontend entry**: Review [`frontend/src/main.tsx`](frontend/src/main.tsx) for React setup
- **Type checking**: Run `cd frontend && pnpm run type-check` (if available) or `tsc --noEmit`

### Relevant Configuration Files

- [`build/config.yml`](build/config.yml) - Wails app metadata (name, bundle identifier, version) and dev mode settings
- [`Taskfile.yml`](Taskfile.yml) and [`build/`](build/) - Wails build tasks per platform
- [`frontend/vite.config.ts`](frontend/vite.config.ts) - Vite bundler configuration
- [`frontend/tsconfig.json`](frontend/tsconfig.json) - TypeScript compiler options
- [`main.go`](main.go) - Application entry point and Wails initialization
- [`app.go`](app.go) - Main application struct with exposed methods

## Future Enhancements

- Rule management UI
- Download history view
- Progress indicators for active downloads
- Settings panel
- File browser integration
