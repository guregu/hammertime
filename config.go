package hammertime

import (
	"io"
	"io/fs"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

type Option func(*WASI)

// WithArgs sets the command line arguments to args.
func WithArgs(args []string) Option {
	return func(wasi *WASI) {
		wasi.args = slices.Clone(args)
	}
}

// WithEnv sets the environment to env, a map of names to values.
func WithEnv(env map[string]string) Option {
	return func(wasi *WASI) {
		if wasi.env == nil {
			wasi.env = make(map[string]string)
		}
		maps.Copy(wasi.env, env)
	}
}

// WithFS uses the given filesystem.
func WithFS(fsys fs.FS) Option {
	return func(wasi *WASI) {
		wasi.fs = fsys
	}
}

// WithClock sets the clock.
// TODO: clock types.
func WithClock(clock Clock) Option {
	return func(wasi *WASI) {
		wasi.clock = clock
	}
}

// WithStdout sets standard output to w.
func WithStdout(w io.Writer) Option {
	return func(wasi *WASI) {
		wasi.stdout = w
	}
}

// WithStdout sets standard error to w.
func WithStderr(w io.Writer) Option {
	return func(wasi *WASI) {
		wasi.stderr = w
	}
}

// WithDebug enables spammy debug logs.
func WithDebug(debug bool) Option {
	return func(wasi *WASI) {
		wasi.debug = debug
	}
}
