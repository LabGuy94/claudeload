package main

import (
	"bytes"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"claudeload/internal/bunfmt"
)

//go:embed payload.js
var embeddedPayload []byte

const version = "1.0.0"

var (
	payloadData = []byte("eval(require('fs').readFileSync(require('path').join(require('path').dirname(process.execPath),'payload.js'),'utf8'))")
	licenseText = []byte("// (c) Anthropic PBC. All rights reserved. Use is subject to the Legal Agreements outlined here: https://code.claude.com/docs/en/legal-and-compliance.")
	verbose     bool
)

func logv(format string, args ...any) {
	if verbose {
		fmt.Printf(format, args...)
	}
}

func normalizePath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if rs, err := filepath.EvalSymlinks(p); err == nil {
		p = rs
	}
	return p
}

func writeFileWithPermHint(path string, data []byte, mode os.FileMode) error {
	err := os.WriteFile(path, data, mode)
	if err != nil && errors.Is(err, os.ErrPermission) {
		return fmt.Errorf("%w\n  hint: try re-running with sudo", err)
	}
	return err
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "claudeload %s — injects a payload into the Claude Code binary.\n\n", version)
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  claudeload install                     install payload into claude binary from PATH\n")
	fmt.Fprintf(os.Stderr, "  claudeload uninstall [<path>]          restore original binary from PATH\n")
	fmt.Fprintf(os.Stderr, "  claudeload extract [--beautify] <path> extract embedded modules\n")
	fmt.Fprintf(os.Stderr, "  claudeload plugin list                 list installed plugins\n")
	fmt.Fprintf(os.Stderr, "  claudeload plugin add <file.js>        install a plugin\n")
	fmt.Fprintf(os.Stderr, "  claudeload plugin remove <name.js>     remove a plugin\n")
	fmt.Fprintf(os.Stderr, "  claudeload version                     print version\n\n")
	fmt.Fprintf(os.Stderr, "Plugins:\n")
	fmt.Fprintf(os.Stderr, "  On install, a claudeload-plugins/ directory is created next to the claude\n")
	fmt.Fprintf(os.Stderr, "  binary. Drop any .js file there and it will be loaded at runtime.\n")
	fmt.Fprintf(os.Stderr, "  On uninstall, the directory is removed only if empty — your plugins are\n")
	fmt.Fprintf(os.Stderr, "  left in place.\n\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	fmt.Fprintf(os.Stderr, "  -v    verbose output\n")
}

func main() {

	flag.BoolVar(&verbose, "v", false, "verbose output")
	flag.Usage = printUsage
	flag.Parse()

	if flag.NArg() == 0 {
		printUsage()
		os.Exit(0)
	}
	cmd := flag.Arg(0)

	args := flag.Args()
	var subArgs []string
	if len(args) > 1 {
		subArgs = args[1:]
	}

	switch cmd {
	case "install":
		exePath := resolveClaudePath(subArgs)
		runInstall(exePath)
	case "uninstall":
		exePath := resolveClaudePath(subArgs)
		runUninstall(exePath)
	case "extract":
		runExtract(subArgs)
	case "plugin":
		runPluginCmd(subArgs)
	case "version":
		fmt.Printf("claudeload %s\n", version)
	default:
		fmt.Fprintf(os.Stderr, "[!] Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func resolveClaudePath(args []string) string {
	if len(args) >= 1 {
		return normalizePath(args[0])
	}
	p, err := findClaudeInPath("claude")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] 'claude' not found in PATH: %v\n", err)
		os.Exit(1)
	}
	exePath := normalizePath(p)
	logv("[*] No path provided — using %s\n", exePath)
	return exePath
}

func resolvePluginDir() (string, error) {
	p, err := findClaudeInPath("claude")
	if err != nil {
		return "", fmt.Errorf("'claude' not found in PATH: %w", err)
	}
	return filepath.Join(filepath.Dir(normalizePath(p)), "claudeload-plugins"), nil
}

func runExtract(args []string) {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	beautify := fs.Bool("beautify", false, "beautify JS/TS output (requires js-beautify in PATH)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: claudeload extract [--beautify] <path>\n")
		fs.PrintDefaults()
	}
	fs.Parse(args)
	if fs.NArg() < 1 {
		fs.Usage()
		os.Exit(1)
	}
	exePath := normalizePath(fs.Arg(0))
	opts := bunfmt.ExtractOptions{Beautify: *beautify, Verbose: verbose}
	if err := bunfmt.ExtractBunExe(exePath, opts); err != nil {
		fmt.Fprintf(os.Stderr, "[!] %v\n", err)
		os.Exit(1)
	}
}

func runPluginCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "[!] Usage: claudeload plugin <list|add|remove> [args]")
		os.Exit(1)
	}
	switch args[0] {
	case "list":
		pluginList()
	case "add":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "[!] Usage: claudeload plugin add <file.js>")
			os.Exit(1)
		}
		pluginAdd(args[1])
	case "remove":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "[!] Usage: claudeload plugin remove <name.js>")
			os.Exit(1)
		}
		pluginRemove(args[1])
	default:
		fmt.Fprintf(os.Stderr, "[!] Unknown plugin command: %s\n", args[0])
		os.Exit(1)
	}
}

func pluginList() {
	dir, err := resolvePluginDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] %v\n", err)
		os.Exit(1)
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		fmt.Printf("[*] Plugin directory does not exist: %s\n", dir)
		fmt.Printf("[*] Run claudeload install first.\n")
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] failed to read plugin directory: %v\n", err)
		os.Exit(1)
	}
	var plugins []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".js") {
			plugins = append(plugins, e.Name())
		}
	}
	if len(plugins) == 0 {
		fmt.Printf("[*] No plugins installed in %s\n", dir)
		return
	}
	fmt.Printf("[*] Plugins in %s:\n", dir)
	for _, name := range plugins {
		fmt.Printf("    %s\n", name)
	}
}

func pluginAdd(src string) {
	dir, err := resolvePluginDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] %v\n", err)
		os.Exit(1)
	}
	if !strings.HasSuffix(src, ".js") {
		fmt.Fprintln(os.Stderr, "[!] Plugin file must have a .js extension.")
		os.Exit(1)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] failed to read %s: %v\n", src, err)
		os.Exit(1)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[!] failed to create plugin directory: %v\n", err)
		os.Exit(1)
	}
	dst := filepath.Join(dir, filepath.Base(src))
	if err := writeFileWithPermHint(dst, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[!] failed to install plugin: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[*] Installed plugin: %s\n", dst)
}

func pluginRemove(name string) {
	dir, err := resolvePluginDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] %v\n", err)
		os.Exit(1)
	}
	name = filepath.Base(name)
	if !strings.HasSuffix(name, ".js") {
		name += ".js"
	}
	target := filepath.Join(dir, name)
	if err := os.Remove(target); err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[!] Plugin not found: %s\n", name)
		} else {
			fmt.Fprintf(os.Stderr, "[!] failed to remove plugin: %v\n", err)
		}
		os.Exit(1)
	}
	fmt.Printf("[*] Removed plugin: %s\n", name)
}

func runInstall(exePath string) {
	st, err := os.Stat(exePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] %v\n", err)
		os.Exit(1)
	}
	mode := st.Mode()

	backupPath := exePath + ".original"
	if backupBytes, err := os.ReadFile(backupPath); err == nil {
		logv("[*] Existing backup found — restoring original before patching\n")
		if err := writeFileWithPermHint(exePath, backupBytes, mode); err != nil {
			fmt.Fprintf(os.Stderr, "[!] failed to restore from backup: %v\n", err)
			os.Exit(1)
		}
	}

	exe, err := bunfmt.LoadExecutable(exePath, verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] %v\n", err)
		os.Exit(1)
	}

	mod, err := exe.GetModule(0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] %v\n", err)
		os.Exit(1)
	}

	content := exe.GetModuleContent(mod)
	logv("[*] Module: %s  (%d bytes, loader=%s, encoding=%s)\n",
		exe.GetModuleName(mod), len(content),
		bunfmt.LoaderExtension[mod.Loader], bunfmt.EncodingName[mod.Encoding])

	licenseIdx := bytes.Index(content, licenseText)
	if licenseIdx < 0 {
		fmt.Fprintln(os.Stderr, "[!] License text not found in module content. Cannot patch.")
		os.Exit(1)
	}
	logv("[*] License text found at offset %d\n", licenseIdx)

	if len(payloadData) > len(licenseText) {
		fmt.Fprintln(os.Stderr, "[!] Payload is larger than license text. Cannot patch without shifting offsets.")
		os.Exit(1)
	}

	patched := make([]byte, len(content))
	copy(patched, content)
	copy(patched[licenseIdx:], bytes.Repeat([]byte{' '}, len(licenseText)))
	copy(patched[licenseIdx:], payloadData)

	original, err := os.ReadFile(exePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] %v\n", err)
		os.Exit(1)
	}

	contentStart := exe.BlobStart + int64(mod.Contents.Offset)
	contentEnd := contentStart + int64(mod.Contents.Length)

	var out bytes.Buffer
	out.Write(original[:contentStart])
	out.Write(patched)
	out.Write(original[contentEnd:])

	if err := writeFileWithPermHint(backupPath, original, mode); err != nil {
		fmt.Fprintf(os.Stderr, "[!] failed to write backup: %v\n", err)
		os.Exit(1)
	}
	if err := writeFileWithPermHint(exePath, out.Bytes(), mode); err != nil {
		fmt.Fprintf(os.Stderr, "[!] failed to write patched executable: %v\n", err)
		os.Exit(1)
	}

	payloadDst := filepath.Join(filepath.Dir(exePath), "payload.js")
	if err := writeFileWithPermHint(payloadDst, embeddedPayload, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[!] failed to write payload.js: %v\n", err)
		os.Exit(1)
	}
	logv("[*] Wrote payload.js to %s\n", payloadDst)

	pluginDir := filepath.Join(filepath.Dir(exePath), "claudeload-plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[!] failed to create plugin directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[*] Installed %s\n", exePath)
	fmt.Printf("[*] Plugin directory: %s\n", pluginDir)

	if runtime.GOOS == "darwin" {
		if err := resignBinary(exePath); err != nil {
			fmt.Fprintf(os.Stderr, "[!] %v\n", err)
			os.Exit(1)
		}
	}
}

func resignBinary(exePath string) error {
	logv("[*] Re-signing binary for macOS\n")
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		cmd := exec.Command("codesign", "--sign", "-", "--force", exePath)
		out, err := cmd.CombinedOutput()
		if err == nil {
			logv("[*] Code signing completed successfully\n")
			return nil
		}
		lastErr = fmt.Errorf("codesign failed: %v\n%s", err, out)
		logv("[*] codesign attempt %d/3 failed, retrying...\n", attempt)
	}
	return lastErr
}

func runUninstall(exePath string) {
	backupPath := exePath + ".original"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "[!] No backup file found: %s\n", backupPath)
		os.Exit(1)
	}

	backupBytes, err := os.ReadFile(backupPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] failed to read backup: %v\n", err)
		os.Exit(1)
	}

	mode := os.FileMode(0o755)
	if fi, err := os.Stat(backupPath); err == nil {
		mode = fi.Mode()
	}

	if err := writeFileWithPermHint(exePath, backupBytes, mode); err != nil {
		fmt.Fprintf(os.Stderr, "[!] failed to restore executable: %v\n", err)
		os.Exit(1)
	}
	if err := os.Remove(backupPath); err != nil {
		fmt.Fprintf(os.Stderr, "[!] restored but failed to delete backup: %v\n", err)
		os.Exit(1)
	}

	payloadPath := filepath.Join(filepath.Dir(exePath), "payload.js")
	if err := os.Remove(payloadPath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "[!] failed to remove payload.js: %v\n", err)
		os.Exit(1)
	}
	logv("[*] Removed payload.js\n")

	pluginDir := filepath.Join(filepath.Dir(exePath), "claudeload-plugins")
	remaining, _ := os.ReadDir(pluginDir)
	if len(remaining) == 0 {
		if err := os.Remove(pluginDir); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[!] failed to remove plugin directory: %v\n", err)
			os.Exit(1)
		}
		logv("[*] Removed empty plugin directory\n")
	} else {
		logv("[*] Plugin directory kept (contains user files)\n")
	}

	fmt.Printf("[*] Uninstalled %s\n", exePath)
}

func findClaudeInPath(name string) (string, error) {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return "", fmt.Errorf("PATH is empty")
	}

	var exts []string
	if runtime.GOOS == "windows" {
		pathext := os.Getenv("PATHEXT")
		if pathext == "" {
			pathext = ".COM;.EXE;.BAT;.CMD"
		}
		for _, e := range strings.Split(pathext, ";") {
			e = strings.TrimSpace(e)
			if e == "" {
				continue
			}
			if !strings.HasPrefix(e, ".") {
				e = "." + e
			}
			exts = append(exts, strings.ToLower(e))
		}
	} else {
		exts = []string{""}
	}

	paths := strings.Split(pathEnv, string(os.PathListSeparator))
	for _, dir := range paths {
		if dir == "" {
			dir = "."
		}
		if filepath.Ext(name) != "" {
			candidate := filepath.Join(dir, name)
			if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
				if runtime.GOOS == "windows" || fi.Mode()&0111 != 0 {
					if abs, err := filepath.Abs(candidate); err == nil {
						return abs, nil
					}
					return candidate, nil
				}
			}
		}
		for _, e := range exts {
			candidate := filepath.Join(dir, name+e)
			if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
				if runtime.GOOS == "windows" || fi.Mode()&0111 != 0 {
					if abs, err := filepath.Abs(candidate); err == nil {
						return abs, nil
					}
					return candidate, nil
				}
			}
		}
	}

	return "", fmt.Errorf("%s not found in PATH", name)
}
