package hammertime

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bytecodealliance/wasmtime-go/v11"
)

func TestWASI(t *testing.T) {
	clock := FixedClock(time.Unix(1690674910, 239502000))
	cases := []struct {
		filename string
		stdout   string
	}{
		{"args.wasm", "2\n0: hello\n1: world\n"},
		{"env.wasm", "it works\n"},
		{"clock.wasm", "1690674910 239502000\n"},
		{"read.wasm", "hello world!"},
		{"dir.wasm", "a.txt\nb.txt\n"},
		{"echo.wasm", "is this thing on?\n"},
	}

	for _, testcase := range cases {
		t.Run(testcase.filename, func(t *testing.T) {
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

			stdin := strings.NewReader("is this thing on?\n")
			stdout := new(bytes.Buffer)
			stderr := new(bytes.Buffer)
			wasi := NewWASI(
				WithArgs([]string{"hello", "world"}),
				WithEnv(map[string]string{"TEST": "it works"}),
				WithClock(clock),
				WithStdin(stdin),
				WithStdout(stdout),
				WithStderr(stderr),
				WithFS(os.DirFS("testdata")),
				WithDebug(true),
			)
			if err := wasi.Link(store, linker); err != nil {
				t.Fatal(err)
			}

			instance, err := linker.Instantiate(store, module)
			if err != nil {
				t.Fatal(err)
			}

			t.Log("running:", testcase.filename)
			start := instance.GetFunc(store, "_start")
			start.Call(store)

			if got := stdout.String(); got != testcase.stdout {
				t.Error("bad stdout. want:", testcase.stdout, "got:", got)
			}
		})
	}
}
