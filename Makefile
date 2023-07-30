clean:
	rm -f testdata/*.wasm

all: testdata/args.wasm testdata/env.wasm testdata/clock.wasm testdata/read.wasm

testdata/%.wasm: testdata/%.c
	$(WASI_CC) testdata/$*.c -o testdata/$*.wasm -g

testdata/%.wat: testdata/%.wasm
	wasm2wat testdata/$*.wasm > testdata/$*.wat
