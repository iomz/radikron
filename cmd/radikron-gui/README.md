# Radikron GUI

This is the graphical user interface for Radikron, built with [Wails v2](https://wails.io).

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

- Go 1.20+
- Node.js and pnpm
- TypeScript 5.3+
- React 18.2+
- Tailwind CSS 4.x
- shadcn/ui components
- Wails v2 CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### Setup

1. Install frontend dependencies:
```bash
cd frontend
pnpm install
```

2. Run in development mode:
```bash
wails dev
```

This will:
- Start the Wails dev server
- Launch the application window
- Enable hot-reload for frontend changes

### Building

Build the application:
```bash
wails build
```

This creates platform-specific binaries in `build/bin/`.

## Architecture

### Structure

```
cmd/radikron-gui/
├── main.go          # Wails entry point
├── app.go           # App struct with exposed methods
├── wails.json       # Wails configuration
└── frontend/
    ├── index.html   # HTML entry point
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

### Events

- `monitoring-started` - Emitted when monitoring starts
- `monitoring-stopped` - Emitted when monitoring stops
- `download-started` - Emitted when a download begins
- `download-completed` - Emitted when a download completes
- `download-failed` - Emitted when a download fails
- `config-loaded` - Emitted when configuration is loaded

## Integration with Core Package

The GUI uses the same core packages as the CLI:

- `github.com/iomz/radikron` - Core download and asset logic
- `github.com/iomz/radikron/internal/config` - Configuration management

This ensures:
- ✅ No code duplication
- ✅ Bug fixes benefit both interfaces
- ✅ Consistent behavior between CLI and GUI

## Future Enhancements

- Rule management UI
- Download history view
- Progress indicators for active downloads
- Settings panel
- File browser integration

