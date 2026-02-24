package bunfmt

// Trailer is the magic byte sequence appended by bun build --compile.
var Trailer = []byte("\n---- Bun! ----\n")

const (
	OffsetsStructSize = 32
	ModuleStructSize  = 52
	ChunkSize         = 4096
)

// LoaderExtension maps the Loader enum (u8) to a file extension.
// Source: bun.options.Loader
var LoaderExtension = map[uint8]string{
	0:  ".jsx",
	1:  ".js",
	2:  ".ts",
	3:  ".tsx",
	4:  ".css",
	5:  ".bin", // file (opaque binary)
	6:  ".json",
	7:  ".jsonc",
	8:  ".toml",
	9:  ".wasm",
	10: ".node", // napi
	11: ".b64",  // base64
	12: ".txt",  // dataurl
	13: ".txt",  // text
	14: ".sh",   // bunsh
	15: ".sqlite",
	16: ".sqlite", // sqlite_embedded
	17: ".html",
	18: ".yaml",
	19: ".json5",
	20: ".md",
}

var EncodingName = map[uint8]string{
	0: "binary",
	1: "latin1",
	2: "utf8",
}
