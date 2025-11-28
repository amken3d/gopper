# Gopper Build System

.PHONY: all clean test rp2040 stm32f4 test-pwm wasm wasm-serve ui

TINYGO = tinygo
TINYGO_CREA8_ROOT = /home/hkeni/sdk/tinygo-versions/tinygo-crea8
TINYGO_CREA8 = TINYGOROOT=$(TINYGO_CREA8_ROOT) $(TINYGO_CREA8_ROOT)/bin/tinygo

# Default target
all: rp2040

# Build for RP2040 (Raspberry Pi Pico)
rp2040:
	$(TINYGO) build -target=pico -size=short -o build/gopper-rp2040.uf2 ./targets/rp2040

# Build for RP2350 (generic - metro-rp2350 target)
rp2350:
	$(TINYGO) build -target=metro-rp2350 -size=short -o build/gopper-rp2350.uf2 ./targets/rp2350

# Build for RP2350 (Pico2 board)
rp2350-pico2:
	$(TINYGO) build -target=pico2 -size=short -o build/gopper-pico2.uf2 ./targets/rp2350

# Build for Crea8 board (RP2350B-based) - uses rp2350 target
crea8:
	$(TINYGO_CREA8) build -target=amken-crea8 -size=short -o build/gopper-crea8.uf2 ./targets/rp2350

# Flash to Crea8 board
flash-crea8:
	$(TINYGO_CREA8) flash -target=amken-crea8 -size=short ./targets/rp2350

# Build blink test for Crea8 (basic USB/LED test)
test-blink-crea8:
	$(TINYGO_CREA8) build -target=amken-crea8 -size=short -o build/blink-test-crea8.uf2 ./test/blink_test

# Flash blink test to Crea8
flash-blink-crea8:
	$(TINYGO_CREA8) flash -target=amken-crea8 -size=short ./test/blink_test

# Build USB debug test for Crea8
test-usb-debug-crea8:
	$(TINYGO_CREA8) build -target=amken-crea8 -size=short -o build/usb-debug-crea8.uf2 ./test/usb_debug

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

# Test 7-axis stepper for Crea8 board (uses custom TinyGo with Crea8 target)
test-7axis-crea8:
	$(TINYGO_CREA8) build -target=amken-crea8 -size=short -o build/7axis-crea8.uf2 ./test/pio_7axis

# Flash 7-axis test to Crea8 board
flash-7axis-crea8:
	$(TINYGO_CREA8) flash -target=amken-crea8 -size=short ./test/pio_7axis

# Test 7-axis stepper (RP2040/Pico for testing)
test-7axis:
	$(TINYGO) build -target=pico -size=short -o build/7axis-test.uf2 ./test/pio_7axis

# Test 7-axis stepper (RP2350 - uses metro-rp2350 target)
test-7axis-rp2350:
	$(TINYGO) build -target=metro-rp2350 -size=short -o build/7axis-rp2350.uf2 ./test/pio_7axis

# Test coordinated multi-axis motion for Crea8 board
test-coordinated-crea8:
	$(TINYGO_CREA8) build -target=amken-crea8 -size=short -o build/coordinated-crea8.uf2 ./test/pio_coordinated

# Flash coordinated test to Crea8 board
flash-coordinated-crea8:
	$(TINYGO_CREA8) flash -target=amken-crea8 -size=short ./test/pio_coordinated

# Test coordinated motion (RP2040/Pico)
test-coordinated:
	$(TINYGO) build -target=pico -size=short -o build/coordinated-test.uf2 ./test/pio_coordinated

# Test PIO stepper V2 (with AssemblerV1 and side-set)
test-pio-v2:
	$(TINYGO) build -target=pico -size=short -o build/pio-stepper-v2.uf2 ./test/pio_stepper_v2

# Test PIO stepper V2 for RP2350
test-pio-v2-rp2350:
	$(TINYGO) build -target=metro-rp2350 -size=short -o build/pio-stepper-v2-rp2350.uf2 ./test/pio_stepper_v2

# Test stepper backends (PIO vs GPIO comparison)
test-stepper-backends:
	$(TINYGO) build -target=pico -size=short -o build/stepper-backends-test.uf2 ./test/stepper_backends

# Test GPIO stepper backend only
test-gpio-stepper:
	$(TINYGO) build -target=pico -size=short -o build/gpio-stepper-test.uf2 ./test/gpio_stepper

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