package bunfmt

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

type ExecutableData struct {
	Blob         []byte
	ModulesBytes []byte
	BlobStart    int64
	TrailerPos   int64
	Offsets      OffsetsStruct
	NumModules   int
}

func FindBunTrailer(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return -1, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return -1, err
	}
	fileSize := fi.Size()
	trailerLen := int64(len(Trailer))

	pos := fileSize
	for pos > 0 {
		readLen := int64(ChunkSize)
		if readLen > pos {
			readLen = pos
		}
		pos -= readLen

		if _, err := f.Seek(pos, io.SeekStart); err != nil {
			return -1, err
		}
		chunk := make([]byte, readLen)
		if _, err := io.ReadFull(f, chunk); err != nil {
			return -1, err
		}

		if idx := bytes.LastIndex(chunk, Trailer); idx >= 0 {
			return pos + int64(idx), nil
		}

		if pos < trailerLen {
			break
		}
		pos += trailerLen
	}

	return -1, nil
}

func LoadExecutable(path string, verbose bool) (*ExecutableData, error) {
	logv := func(format string, args ...any) {
		if verbose {
			fmt.Printf(format, args...)
		}
	}

	logv("[*] Analyzing %s...\n", path)

	trailerPos, err := FindBunTrailer(path)
	if err != nil {
		return nil, fmt.Errorf("scanning for trailer: %w", err)
	}
	if trailerPos < 0 {
		return nil, fmt.Errorf("could not find Bun trailer signature — is this a 'bun build --compile' executable?")
	}
	logv("[*] Found trailer at offset: %d\n", trailerPos)

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	offsetsStart := trailerPos - int64(OffsetsStructSize)
	if _, err := f.Seek(offsetsStart, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seeking to offsets struct: %w", err)
	}
	offsetsBuf := make([]byte, OffsetsStructSize)
	if _, err := io.ReadFull(f, offsetsBuf); err != nil {
		return nil, fmt.Errorf("reading offsets struct: %w", err)
	}

	offsets, err := ReadOffsetsStruct(offsetsBuf)
	if err != nil {
		return nil, err
	}

	logv("\n--- Offsets Struct ---\n")
	logv("  byte_count:            %d bytes\n", offsets.ByteCount)
	logv("  modules_ptr:           offset=%d, length=%d\n", offsets.ModulesPtr.Offset, offsets.ModulesPtr.Length)
	logv("  entry_point_id:        %d\n", offsets.EntryPointID)
	logv("  compile_exec_argv_ptr: offset=%d, length=%d\n", offsets.CompileExecArgvPtr.Offset, offsets.CompileExecArgvPtr.Length)
	logv("  flags:                 %b (%d)\n", offsets.Flags, offsets.Flags)

	blobStart := offsetsStart - int64(offsets.ByteCount)
	if blobStart < 0 {
		return nil, fmt.Errorf("calculated blob start is negative — file may be corrupted")
	}
	logv("  blob_start:            %d\n", blobStart)

	if _, err := f.Seek(blobStart, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seeking to blob: %w", err)
	}
	blob := make([]byte, offsets.ByteCount)
	if _, err := io.ReadFull(f, blob); err != nil {
		return nil, fmt.Errorf("unexpected EOF reading data blob: %w", err)
	}

	modulesBytes := offsets.ModulesPtr.Read(blob)
	numModules := len(modulesBytes) / ModuleStructSize
	logv("  module_count:          %d\n", numModules)

	argvData := offsets.CompileExecArgvPtr.Read(blob)
	if len(argvData) > 0 {
		logv("  compile_exec_argv:     %s\n", strings.ToValidUTF8(string(argvData), "?"))
	}

	return &ExecutableData{
		Blob:         blob,
		ModulesBytes: modulesBytes,
		BlobStart:    blobStart,
		TrailerPos:   trailerPos,
		Offsets:      offsets,
		NumModules:   numModules,
	}, nil
}

func (exe *ExecutableData) GetModule(index int) (ModuleStruct, error) {
	if index < 0 || index >= exe.NumModules {
		return ModuleStruct{}, fmt.Errorf("module index %d out of range [0, %d)", index, exe.NumModules)
	}
	start := index * ModuleStructSize
	end := start + ModuleStructSize
	if end > len(exe.ModulesBytes) {
		return ModuleStruct{}, fmt.Errorf("module index %d is out of bounds in modules data (corrupt binary?)", index)
	}
	return ReadModuleStruct(exe.ModulesBytes[start:end])
}

func (exe *ExecutableData) GetModuleName(m ModuleStruct) string {
	raw := m.Name.Read(exe.Blob)
	return strings.ToValidUTF8(string(raw), "?")
}

func (exe *ExecutableData) GetModuleContent(m ModuleStruct) []byte {
	return m.Contents.Read(exe.Blob)
}

func (exe *ExecutableData) FindModuleByName(name string) (ModuleStruct, int, error) {
	for i := 0; i < exe.NumModules; i++ {
		m, err := exe.GetModule(i)
		if err != nil {
			return ModuleStruct{}, -1, err
		}
		if exe.GetModuleName(m) == name {
			return m, i, nil
		}
	}
	return ModuleStruct{}, -1, fmt.Errorf("module %q not found", name)
}
