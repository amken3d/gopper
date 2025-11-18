# Gopper Build System

.PHONY: all clean test rp2040 stm32f4 test-pwm wasm wasm-serve ui

TINYGO = tinygo

# Default target
all: rp2040

# Build for RP2040 (Raspberry Pi Pico)
rp2040:
	$(TINYGO) build -target=pico -size=short -o build/gopper-rp2040.uf2 ./targets/rp2040

# Build for STM32F4
stm32f4:
	$(TINYGO) build -target=nucleo-f446re -size=short -o build/gopper-stm32f4.hex ./targets/stm32f4

# Build PWM test for RP2040
test-pwm:
	$(TINYGO) build -target=pico -size=short -o build/pwm-test-rp2040.uf2 ./test/pwm

# Run tests (protocol package only - core tests TODO)
test:
	go test -v ./protocol/...

# Clean build artifacts
clean:
	rm -rf build/

# Create build directory
build:
	mkdir -p build

# Build WebAssembly UI
wasm: build
	@echo "Building Wasm module..."
	$(TINYGO) build -o ui/web/gopper.wasm -target=wasm ./ui/wasm
	@echo "Copying wasm_exec.js..."
	cp $$($(TINYGO) env TINYGOROOT)/targets/wasm_exec.js ui/web/
	@echo "Wasm build complete! Files in ui/web/"

# Build UI (alias for wasm)
ui: wasm

# Serve the UI with a local web server
wasm-serve: wasm
	@echo "Starting web server at http://localhost:8080"
	@echo "Press Ctrl+C to stop"
	@which python3 > /dev/null && python3 -m http.server 8080 -d ui/web || \
	 (which python > /dev/null && python -m SimpleHTTPServer 8080 -d ui/web) || \
	 echo "Error: Python not found. Please install Python or use another web server."