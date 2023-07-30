# hammertime

Do you want to use the excellent [wasmtime-go](https://github.com/bytecodealliance/wasmtime-go) Wasm runtime library, but are missing some features like [capturing stdout or setting stdin](https://github.com/bytecodealliance/wasmtime-go/issues/34)?

This library is a WASI implementation in Go for wasmtime-go. Its goal is to integrate Wasm more deeply with Go but still take advantage of Wasmtime's speedy runtime.

#### Status

Rough proof of concept targeting `wasi_snapshot_preview1`. If this project proves to be useful, I'm thinking a code generation approach targeting preview2 would be the next step.

TL;DR: Alpha! 🤠

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

# Testing

To build/run the test files, install WASI SDK, then do something like:

```console
$ make WASI_CC=/path/to/wasi-sdk-20.0/bin/clang all
$ go test -v
```
