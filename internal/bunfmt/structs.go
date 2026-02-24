package bunfmt

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type StringPointer struct {
	Offset uint32
	Length uint32
}

func (sp StringPointer) Read(blob []byte) []byte {
	if sp.Length == 0 {
		return []byte{}
	}
	end := sp.Offset + sp.Length
	if int(end) > len(blob) {
		return nil
	}
	return blob[sp.Offset:end]
}

// OffsetsStruct represents the 32-byte struct immediately before the trailer.
// Layout (little-endian):
//
//	byte_count:             u64  (8)
//	modules_ptr.offset:     u32  (4)
//	modules_ptr.length:     u32  (4)
//	entry_point_id:         u32  (4)
//	compile_exec_argv_ptr.offset: u32 (4)
//	compile_exec_argv_ptr.length: u32 (4)
//	flags:                  u32  (4)
//
// Total: 32 bytes
type OffsetsStruct struct {
	ByteCount          uint64
	ModulesPtr         StringPointer
	EntryPointID       uint32
	CompileExecArgvPtr StringPointer
	Flags              uint32
}

// wireOffsets is a flat struct matching the exact 32-byte wire layout.
type wireOffsets struct {
	ByteCount        uint64
	ModOff, ModLen   uint32
	EntryPointID     uint32
	ArgvOff, ArgvLen uint32
	Flags            uint32
}

// ReadOffsetsStruct deserializes 32 bytes into an OffsetsStruct.
func ReadOffsetsStruct(data []byte) (OffsetsStruct, error) {
	if len(data) < OffsetsStructSize {
		return OffsetsStruct{}, fmt.Errorf("offsets data too short: %d < %d", len(data), OffsetsStructSize)
	}
	var w wireOffsets
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &w); err != nil {
		return OffsetsStruct{}, fmt.Errorf("parsing offsets struct: %w", err)
	}
	return OffsetsStruct{
		ByteCount:          w.ByteCount,
		ModulesPtr:         StringPointer{w.ModOff, w.ModLen},
		EntryPointID:       w.EntryPointID,
		CompileExecArgvPtr: StringPointer{w.ArgvOff, w.ArgvLen},
		Flags:              w.Flags,
	}, nil
}

// ModuleStruct represents one 52-byte CompiledModuleGraphFile entry.
// Layout (little-endian):
//
//	name:                  StringPointer (8)
//	contents:              StringPointer (8)
//	sourcemap:             StringPointer (8)
//	bytecode:              StringPointer (8)
//	module_info:           StringPointer (8)
//	bytecode_origin_path:  StringPointer (8)
//	encoding:              u8
//	loader:                u8
//	module_format:         u8
//	side:                  u8
//
// Total: 52 bytes
type ModuleStruct struct {
	Name               StringPointer
	Contents           StringPointer
	SourceMap          StringPointer
	Bytecode           StringPointer
	ModuleInfo         StringPointer
	BytecodeOriginPath StringPointer
	Encoding           uint8
	Loader             uint8
	ModuleFormat       uint8
	Side               uint8
}

type wireModule struct {
	NameOff, NameLen               uint32
	ContentsOff, ContentsLen       uint32
	MapOff, MapLen                 uint32
	BytecodeOff, BytecodeLen       uint32
	ModInfoOff, ModInfoLen         uint32
	OriginOff, OriginLen           uint32
	Encoding, Loader, ModFmt, Side uint8
}

// ReadModuleStruct deserializes 52 bytes into a ModuleStruct.
func ReadModuleStruct(data []byte) (ModuleStruct, error) {
	if len(data) < ModuleStructSize {
		return ModuleStruct{}, fmt.Errorf("module data too short: %d < %d", len(data), ModuleStructSize)
	}
	var w wireModule
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &w); err != nil {
		return ModuleStruct{}, fmt.Errorf("parsing module struct: %w", err)
	}
	return ModuleStruct{
		Name:               StringPointer{w.NameOff, w.NameLen},
		Contents:           StringPointer{w.ContentsOff, w.ContentsLen},
		SourceMap:          StringPointer{w.MapOff, w.MapLen},
		Bytecode:           StringPointer{w.BytecodeOff, w.BytecodeLen},
		ModuleInfo:         StringPointer{w.ModInfoOff, w.ModInfoLen},
		BytecodeOriginPath: StringPointer{w.OriginOff, w.OriginLen},
		Encoding:           w.Encoding,
		Loader:             w.Loader,
		ModuleFormat:       w.ModFmt,
		Side:               w.Side,
	}, nil
}
