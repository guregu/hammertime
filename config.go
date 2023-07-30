package mywasi

import (
	"io"
	"io/fs"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

type Option func(*WASI)

func WithArgs(args []string) Option {
	return func(wasi *WASI) {
		wasi.args = slices.Clone(args)
	}
}

func WithEnv(env map[string]string) Option {
	return func(wasi *WASI) {
		if wasi.env == nil {
			wasi.env = make(map[string]string)
		}
		maps.Copy(wasi.env, env)
	}
}

func WithFS(fsys fs.FS) Option {
	return func(wasi *WASI) {
		wasi.fs = fsys
	}
}

func WithClock(clock Clock) Option {
	return func(wasi *WASI) {
		wasi.clock = clock
	}
}

func WithStdout(w io.Writer) Option {
	return func(wasi *WASI) {
		wasi.stdout = w
	}
}

func WithStderr(w io.Writer) Option {
	return func(wasi *WASI) {
		wasi.stderr = w
	}
}
