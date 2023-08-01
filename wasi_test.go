package hammertime

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bytecodealliance/wasmtime-go/v11"
	hpos "github.com/hack-pad/hackpadfs/os"
	// _ "github.com/benesch/cgosymbolizer"
)

func TestWASI(t *testing.T) {
	stdinText := "is this thing on?\nhow about this?\n"

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
		{"echo.wasm", stdinText},
		{"mkdir.wasm", "a 0 0\nb 0 0\nc 0 0\nd 0 0\n"},
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

			stdin := strings.NewReader(stdinText)
			stdout := new(bytes.Buffer)
			stderr := new(bytes.Buffer)
			dir, err := filepath.Abs("testdata")
			if err != nil {
				t.Fatal(err)
			}
			dirfs, err := hpos.NewFS().Sub(dir[1:])
			if err != nil {
				t.Fatal(err)
			}
			wasi := NewWASI(
				WithArgs([]string{"hello", "world"}),
				WithEnv(map[string]string{"TEST": "it works"}),
				WithClock(clock),
				WithStdin(stdin),
				WithStdout(stdout),
				WithStderr(stderr),
				WithFS(dirfs),
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
