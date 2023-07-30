ifndef PROGS
PROGS = testdata/args.wasm \
	testdata/env.wasm \
	testdata/clock.wasm \
	testdata/read.wasm \
	testdata/dir.wasm
endif

clean:
	rm -f testdata/*.wasm

all: $(PROGS)

testdata/%.wasm: testdata/%.c
	$(WASI_CC) testdata/$*.c -o testdata/$*.wasm -g

testdata/%.wat: testdata/%.wasm
	wasm2wat testdata/$*.wasm > testdata/$*.wat
