package mywasi

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bytecodealliance/wasmtime-go/v11"
)

func TestWasi(t *testing.T) {
	clock := FixedClock(time.Unix(1690674910, 239502000))
	cases := []struct {
		filename string
		stdout   string
	}{
		{"args.wasm", "2\n0: hello\n1: world\n"},
		{"env.wasm", "it works\n"},
		{"clock.wasm", "1690674910 239502000\n"},
		{"read.wasm", "hello world!"},
	}

	for _, testcase := range cases {
		wasm, err := os.ReadFile(filepath.Join("testdata", testcase.filename))
		if err != nil {
			t.Fatal(err)
		}
		engine := wasmtime.NewEngine()
		store := wasmtime.NewStore(engine)
		module, err := wasmtime.NewModule(engine, wasm)
		if err != nil {
			t.Fatal(err)
		}
		linker := wasmtime.NewLinker(engine)

		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		wasi := NewWASI(
			WithArgs([]string{"hello", "world"}),
			WithEnv(map[string]string{"TEST": "it works"}),
			WithClock(clock),
			WithStdout(stdout),
			WithStderr(stderr),
			WithFS(os.DirFS("testdata")),
		)
		wasi.Define(store, linker)

		instance, err := linker.Instantiate(store, module)
		if err != nil {
			t.Fatal(err)
		}
		start := instance.GetFunc(store, "_start")
		start.Call(store)

		if got := stdout.String(); got != testcase.stdout {
			t.Error("bad stdout. want:", testcase.stdout, "got:", got)
		}
	}
}