# claudeload

A simple tool to inject custom JavaScript plugins into the Claude Code Bun context.

## Install (one‑line)

**macOS/Linux**
```/dev/null/install.sh#L1-1
curl -fsSL https://raw.githubusercontent.com/LabGuy94/claudeload/main/install.sh | sh
```

**Windows (PowerShell)**
```/dev/null/install.ps1#L1-1
iwr -useb https://raw.githubusercontent.com/LabGuy94/claudeload/main/install.ps1 | iex
```

## Build from source
```/dev/null/build.sh#L1-3
go build -o claudeload ./cmd/claudeload
```

## Usage
```/dev/null/usage.txt#L1-6
claudeload install
claudeload uninstall
claudeload extract [--beautify] <path>
claudeload plugin list
claudeload plugin add <file.js>
claudeload plugin remove <name.js>
```

## Plugins
On install, `claudeload-plugins/` is created next to the `claude` binary. Any `.js` files in that directory are loaded at runtime.

## Notes
- For `extract --beautify`, `js-beautify` is required: https://www.npmjs.com/package/js-beautify
- This tool modifies the Claude Code executable on disk. Use responsibly and keep backups.
- Some example plugins are included in `example-plugins`.
