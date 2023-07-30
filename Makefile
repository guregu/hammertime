CC = $(CC)
ifdef WASI_CC
CC = $(WASI_CC)
endif

ifndef PROGS
PROGS = testdata/args.wasm 	\
	testdata/env.wasm 		\
	testdata/clock.wasm 	\
	testdata/read.wasm 		\
	testdata/dir.wasm 		\
	testdata/hello.wasm
endif

.PHONY: test

all: build test

clean:
	rm -f testdata/*.wasm
	go clean

build: $(PROGS)

test: build
	go test -v -timeout 10s -race

testdata/%.wasm: testdata/%.c
	$(CC) testdata/$*.c -o testdata/$*.wasm -g

testdata/%.wat: testdata/%.wasm
	wasm2wat testdata/$*.wasm > testdata/$*.wat
