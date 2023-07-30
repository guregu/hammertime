package hammertime

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bytecodealliance/wasmtime-go/v11"
)

func Example() {
	wasm, err := os.ReadFile(filepath.Join("testdata", "args.wasm"))
	if err != nil {
		panic(err)
	}

	engine := wasmtime.NewEngine()
	store := wasmtime.NewStore(engine)
	module, err := wasmtime.NewModule(engine, wasm)
	if err != nil {
		panic(err)
	}
	linker := wasmtime.NewLinker(engine)

	stdout := new(bytes.Buffer)
	wasi := NewWASI(
		WithArgs([]string{"args.wasm", "hello", "world"}),
		WithStdout(stdout),
	)
	wasi.Link(store, linker)

	instance, err := linker.Instantiate(store, module)
	if err != nil {
		panic(err)
	}

	start := instance.GetFunc(store, "_start")
	start.Call(store)

	// captured stdout:
	fmt.Println(stdout.String())
	// Output: 3
	// 0: args.wasm
	// 1: hello
	// 2: world
}
