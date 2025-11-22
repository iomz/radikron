# Setup Guide for Radikron GUI

This is a proof-of-concept Wails GUI for Radikron. Follow these steps to get it running.

## Prerequisites

- **Node.js**: Required for frontend development. Install from [https://nodejs.org](https://nodejs.org)
- **pnpm**: Install globally via npm:

  ```bash
  npm install -g pnpm
  ```

## Setup Steps

1. **Ensure Go is installed** (if not already installed):
   Make sure Go is installed and available in your PATH. You can download it from [https://go.dev](https://go.dev)

2. **Install Wails v2**:

   ```bash
   go install github.com/wailsapp/wails/v2/cmd/wails@latest
   ```

3. **Install frontend dependencies**:

   ```bash
   cd frontend
   pnpm install
   cd ..
   ```

4. **Add Wails dependency to the main go.mod**:
   From the project root, run:

   ```bash
   go get github.com/wailsapp/wails/v2
   ```

## Development

Run in development mode:

```bash
wails dev
```

This will:

- Start the Wails dev server
- Launch the application window
- Enable hot-reload for frontend changes

## Building

Build the application:

```bash
wails build
```

This creates platform-specific binaries in `build/bin/`.

## Notes

- The GUI uses the same core `radikron` package as the CLI
- Configuration file location defaults to `config.yml` in the current directory
- Both CLI and GUI can coexist and use the same config file
- The monitoring loop runs in a background goroutine, similar to the CLI version

## Troubleshooting

If you encounter issues:

1. **Wails not found**: Make sure `$GOPATH/bin` or `$HOME/go/bin` is in your PATH
2. **Frontend build errors**: Run `pnpm install` in the `frontend` directory
3. **Go module errors**: Run `go mod tidy` from the project root
