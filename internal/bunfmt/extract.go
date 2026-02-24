package bunfmt

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

type ExtractOptions struct {
	Beautify bool //
	Verbose  bool
}

func ExtractBunExe(exePath string, opts ExtractOptions) error {
	logv := func(format string, args ...any) {
		if opts.Verbose {
			fmt.Printf(format, args...)
		}
	}

	exe, err := LoadExecutable(exePath, opts.Verbose)
	if err != nil {
		return err
	}

	outputDir := filepath.Join(".", filepath.Base(exePath)+"_extracted")
	fmt.Printf("[*] Extracting to: %s\n", outputDir)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	count := 0
	for i := 0; i < exe.NumModules; i++ {
		mod, err := exe.GetModule(i)
		if err != nil {
			return err
		}

		name := exe.GetModuleName(mod)
		content := exe.GetModuleContent(mod)

		encName := EncodingName[mod.Encoding]
		if encName == "" {
			encName = fmt.Sprintf("%d", mod.Encoding)
		}
		loaderExt := loaderExtension(mod.Loader)

		logv("\n--- Module [%d] ---\n", i)
		logv("  name:                 %s\n", orEmpty(name))
		logv("  name_ptr:             offset=%d, length=%d\n", mod.Name.Offset, mod.Name.Length)
		logv("  contents_ptr:         offset=%d, length=%d\n", mod.Contents.Offset, mod.Contents.Length)
		logv("  sourcemap_ptr:        offset=%d, length=%d\n", mod.SourceMap.Offset, mod.SourceMap.Length)
		logv("  bytecode_ptr:         offset=%d, length=%d\n", mod.Bytecode.Offset, mod.Bytecode.Length)
		logv("  module_info_ptr:      offset=%d, length=%d\n", mod.ModuleInfo.Offset, mod.ModuleInfo.Length)
		logv("  bytecode_origin_ptr:  offset=%d, length=%d\n", mod.BytecodeOriginPath.Offset, mod.BytecodeOriginPath.Length)
		logv("  encoding:             %s (%d)\n", encName, mod.Encoding)
		logv("  loader:               %s (%d)\n", loaderExt, mod.Loader)
		logv("  module_format:        %d\n", mod.ModuleFormat)
		logv("  side:                 %d\n", mod.Side)

		if len(content) == 0 {
			logv("  -> skipped (0 bytes)\n")
			continue
		}

		savePath := resolveOutputPath(outputDir, name, mod.Loader, i)

		if err := os.MkdirAll(filepath.Dir(savePath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(savePath, content, 0o644); err != nil {
			return err
		}
		logv("  -> saved %d bytes to %s\n", len(content), savePath)
		count++

		if opts.Beautify && mod.Loader <= 3 {
			logv("  -> beautifying %s...\n", filepath.Base(savePath))
			if beautified, err := beautifyJS(content); err == nil {
				outPath := savePath + ".beautified.js"
				if err := os.WriteFile(outPath, beautified, 0o644); err != nil {
					fmt.Fprintf(os.Stderr, "[!] failed to write beautified output: %v\n", err)
				} else {
					logv("  -> beautified output: %s\n", outPath)
				}
			} else {
				logv("  -> beautify skipped: %v\n", err)
			}
		}

		if mod.SourceMap.Length > 0 {
			mapData := mod.SourceMap.Read(exe.Blob)
			if err := os.WriteFile(savePath+".map", mapData, 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "[!] failed to write source map for %s: %v\n", filepath.Base(savePath), err)
			}
		}

		if mod.Bytecode.Length > 0 {
			bcData := mod.Bytecode.Read(exe.Blob)
			if err := os.WriteFile(savePath+".bytecode", bcData, 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "[!] failed to write bytecode for %s: %v\n", filepath.Base(savePath), err)
			}
		}
	}

	fmt.Printf("[*] Extracted %d files.\n", count)
	return nil
}

func resolveOutputPath(outputDir, name string, loader uint8, index int) string {
	clean := name

	for _, prefix := range []string{"/$bunfs/", `B:\~BUN\`} {
		if strings.HasPrefix(clean, prefix) {
			clean = clean[len(prefix):]
			break
		}
	}

	clean = strings.TrimLeft(clean, "/\\")
	if idx := strings.Index(clean, ":"); idx >= 0 {
		clean = strings.TrimLeft(clean[idx+1:], "/\\")
	}

	parts := strings.FieldsFunc(strings.ReplaceAll(clean, "\\", "/"), func(r rune) bool {
		return r == '/'
	})
	sanitized := make([]string, 0, len(parts))
	for _, part := range parts {
		part = sanitizePathComponent(part)
		if part != "" {
			sanitized = append(sanitized, part)
		}
	}

	if len(sanitized) == 0 {
		ext := loaderExtension(loader)
		return filepath.Join(outputDir, fmt.Sprintf("module_%d%s", index, ext))
	}

	joined := filepath.Join(sanitized...)

	base := filepath.Base(joined)
	origExt := filepath.Ext(base)
	if origExt != "" {
		joined = strings.TrimSuffix(joined, origExt) + origExt + loaderExtension(loader)
	} else {
		joined = joined + loaderExtension(loader)
	}

	return filepath.Join(outputDir, joined)
}

func sanitizePathComponent(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '\n' || r == '\r' {
			b.WriteRune('_')
		} else if !unicode.IsPrint(r) || strings.ContainsRune(`\/:*?"<>|`, r) {
			b.WriteRune('_')
		} else {
			b.WriteRune(r)
		}
	}
	result := strings.TrimSpace(b.String())
	if len(result) > 200 {
		result = result[:200]
	}
	return result
}

func loaderExtension(loader uint8) string {
	if ext, ok := LoaderExtension[loader]; ok {
		return ext
	}
	return ".bin"
}

func orEmpty(s string) string {
	if s == "" {
		return "<empty>"
	}
	return s
}

func beautifyJS(src []byte) ([]byte, error) {
	path, err := exec.LookPath("js-beautify")
	if err != nil {
		return nil, fmt.Errorf("js-beautify not found in PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "-")
	cmd.Stdin = strings.NewReader(string(src))
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("js-beautify timed out after 30s")
		}
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("js-beautify: %s", strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}
	return out, nil
}
