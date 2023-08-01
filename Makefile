ifndef PROGS
SRC   = $(wildcard testdata/*.c)
PROGS = $(patsubst %.c,%.wasm,$(SRC))
endif

CC = $(CC)
ifdef WASI_CC
CC = $(WASI_CC)
endif

TESTFLAGS := -v -timeout 10s -race

.PHONY: test

all: build test

clean:
	rm -f testdata/*.wasm testdata/*.wat
	go clean

build: $(PROGS)

wat: $(patsubst %.wasm,%.wat,$(PROGS))

test: build
	go test $(TESTFLAGS)

testdata/%.wasm: testdata/%.c
	$(CC) testdata/$*.c -o testdata/$*.wasm -g

testdata/%.wat: testdata/%.wasm
	wasm2wat testdata/$*.wasm > testdata/$*.wat
