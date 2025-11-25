# Gopper Build System

.PHONY: all clean test rp2040 stm32f4 test-pwm wasm wasm-serve ui host host-test

TINYGO = tinygo
GO = go

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

# Build PIO stepper test for RP2040
test-stepper:
	$(TINYGO) build -target=pico -size=short -o build/stepper-test-rp2040.uf2 ./test/stepper

# Build simple PIO toggle test
test-pio-simple:
	$(TINYGO) build -target=pico -size=short -o build/pio-simple-test.uf2 ./test/pio_simple

# Build PIO stepper test with FIFO
test-pio-stepper:
	$(TINYGO) build -target=pico -size=short -o build/pio-stepper-test.uf2 ./test/pio_stepper

# Test the actual PIOStepperBackend implementation
test-pio-backend:
	$(TINYGO) build -target=pico -size=short -o build/pio-backend-test.uf2 ./test/pio_backend

# Test minimal PIO backend (PULL + toggle only, no OUT)
test-pio-minimal:
	$(TINYGO) build -target=pico -size=short -o build/pio-minimal-test.uf2 ./test/pio_minimal

# Test OUT instruction with toggle
test-pio-out:
	$(TINYGO) build -target=pico -size=short -o build/pio-out-test.uf2 ./test/pio_out_test

# Test simple PIO stepper (RP2040/Pico)
test-simple-stepper:
	$(TINYGO) build -target=pico -size=short -o build/simple-stepper-test.uf2 ./test/pio_simple_stepper

# Test simple PIO stepper (RP2350B)
test-simple-stepper-rp2350b:
	$(TINYGO) build -target=rp2350b -size=short -o build/simple-stepper-rp2350b.uf2 ./test/pio_simple_stepper

# Test 7-axis stepper (RP2040/Pico for testing)
test-7axis:
	$(TINYGO) build -target=pico -size=short -o build/7axis-test.uf2 ./test/pio_7axis

# Test 7-axis stepper (RP2350 - uses metro-rp2350 target)
test-7axis-rp2350:
	$(TINYGO) build -target=metro-rp2350 -size=short -o build/7axis-rp2350.uf2 ./test/pio_7axis

# Test PIO stepper V2 (with AssemblerV1 and side-set)
test-pio-v2:
	$(TINYGO) build -target=pico -size=short -o build/pio-stepper-v2.uf2 ./test/pio_stepper_v2

# Test PIO stepper V2 for RP2350
test-pio-v2-rp2350:
	$(TINYGO) build -target=metro-rp2350 -size=short -o build/pio-stepper-v2-rp2350.uf2 ./test/pio_stepper_v2

# Test stepper backends (PIO vs GPIO comparison)
test-stepper-backends:
	$(TINYGO) build -target=pico -size=short -o build/stepper-backends-test.uf2 ./test/stepper_backends

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

# Build host software (Standard Go)
host: build
	@echo "Building Gopper host..."
	$(GO) build -o build/gopper-host ./host/cmd/gopper-host
	@echo "Host build complete! Binary: build/gopper-host"

# Run host with default device
host-run: host
	@echo "Running Gopper host..."
	./build/gopper-host -device /dev/ttyACM0 -verbose

# Test host (requires MCU connected)
host-test: host
	@echo "Testing host connection..."
	./build/gopper-host -device /dev/ttyACM0