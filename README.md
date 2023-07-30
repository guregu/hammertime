# hammertime [![GoDoc](https://godoc.org/github.com/guregu/hammertime?status.svg)](https://godoc.org/github.com/guregu/hammertime)

`import "github.com/guregu/hammertime"`

Do you want to use the excellent [wasmtime-go](https://github.com/bytecodealliance/wasmtime-go) Wasm runtime library, but are missing some features like [capturing stdout or setting stdin](https://github.com/bytecodealliance/wasmtime-go/issues/34)?

This library is a WASI implementation in Go for wasmtime-go. Its goal is to integrate Wasm more deeply with Go but still take advantage of Wasmtime's speedy runtime.

#### Status

Rough proof of concept targeting `wasi_snapshot_preview1`. If this project proves to be useful, I'm thinking a code generation approach targeting preview2 would be the next step.

TL;DR: Alpha!

- ⛔️ Note that hammertime does not implement the preview1 capabilities model (yet?).
- ☣️ It's also not safe to share WASI instances concurrently or across instances (yet?).
- 😇 Lots of `unsafe`. Needs fuzzing or something.
- 🤠 Experimental. Ideas welcome!

## Features

- Use `fs.FS` for the Wasm filesystem
- `stdin` can be set to an `io.Reader`
- `stdout` and `stderr` can be set to a `io.Writer`
- More experimental stuff coming soon?

| WASI API                  | Vibe   |
|---------------------------|--------|
| args_sizes_get            | 😎     |
| args_get                  | 😎     |
| environ_sizes_get         | 😎     |
| environ_get               | 😎     |
| clock_time_get            | 🧐     |
| fd_close                  | 🧐     |
| fd_fdstat_get             | 🙂     |
| fd_fdstat_set_flags       | 😶‍🌫️     |
| fd_prestat_get            | 🙂     |
| fd_prestat_dir_name       | 😎     |
| fd_filestat_get           | 🧐     |
| fd_seek                   | 🙂     |
| fd_write                  | 🙂     |
| fd_read                   | 🙂     |
| fd_pread                  | 🧐     |
| fd_readdir                | 🙂     |
| path_open                 | 🧐     |
| path_filestat_get         | 🧐     |
| path_readlink             | 😶‍🌫️     |
| path_rename               | 😶‍🌫️     |
| path_create_directory     | 😶‍🌫️     |
| path_remove_directory     | 😶‍🌫️     |
| path_unlink_file          | 😶‍🌫️     |
| poll_oneoff               | 😶‍🌫️     |
| proc_exit                 | 😶‍🌫️     |

#### Legend

|    | Interpretation                 |
| -- | ------------------------------ |
| 😎 | Pretty good                    |
| 🙂 | Not bad                        |
| 🧐 | Needs more work/testing/love   |
| 😶‍🌫️ | Stub/missing                   |

# Usage

See: [Godoc](https://godoc.org/github.com/trealla-prolog/go)

### Quick Start

Imagine we have this C program we want to execute as WebAssembly. It's a simple program that receives a newline-separated list of who to greet via standard input, and writes "hello {name}" to standard output.

```c
int main() {
    char *line = NULL;
    size_t len = 0;
    ssize_t read = 0;
    while ((read = getline(&line, &len, stdin)) != -1) {
        printf("hello %s", line);
    }
    free(line);
    return 0;
}
```

We can embed and execute it in a Go program like so, capturing the output:

```go
import (
    "bytes"
    _ "embed"
    "log"
    "os"

    "github.com/bytecodealliance/wasmtime-go/v11"
    "github.com/guregu/hammertime"
)

//go:embed hello.wasm
var wasmModule []byte // Protip: stuff your modules into your binary with embed

func main() {
    // Standard boilerplate
    engine := wasmtime.NewEngine()
    store := wasmtime.NewStore(engine)
    module, err := wasmtime.NewModule(engine, wasmModule)
    if err != nil {
        panic(err)
    }
    linker := wasmtime.NewLinker(engine)

    // Prepare our input and output
    input := "alice\nbob\n"
    stdin := strings.NewReader(input)
    stdout := new(bytes.Buffer)

    // Set up our custom WASI
    wasi := hammertime.NewWASI(
        WithArgs([]string{"hello.wasm"}),
        WithStdin(stdin),             // Stdin can be any io.Reader
        WithStdout(stdout),           // Capture stdout to a *bytes.Buffer!
        WithFS(os.DirFS("testdata")), // Works with Go's fs.FS! (kind of)
    )
    // Link our WASI
    if err := wasi.Link(store, linker); err != nil {
        panic(err)
    }

    // Use wasmtime as normal
    instance, err := linker.Instantiate(store, module)
    if err != nil {
        panic(err)
    }
    start := instance.GetFunc(store, "_start")
    _, err = start.Call(store)
    if err != nil {
        panic(err)
    }

    // Grab captured stdout data
    output := stdout.String()
    fmt.Println(output)
    // Prints: hello alice
    // hello bob
}
```

This gives us an easy way to communicate with wasm modules.

# Testing

Each testdata/*.c file is a little self-contained C program that tests a WASI feature.

To build/run the test files, [install WASI SDK](https://github.com/WebAssembly/wasi-sdk#install), then do something like:

```console
$ export WASI_CC=/path/to/wasi-sdk-XX.0/bin/clang
$ make clean && make -j8
```
