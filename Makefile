# Gopper Build System

.PHONY: all clean test rp2040 stm32f4 test-multicore test-multicore-pico2

TINYGO = tinygo

# Default target
all: rp2040

# Build for RP2040 (Raspberry Pi Pico)
rp2040:
	$(TINYGO) build -target=pico -size=short -o build/gopper-rp2040.uf2 ./targets/rp2040

# Build for STM32F4
stm32f4:
	$(TINYGO) build -target=nucleo-f446re -size=short -o build/gopper-stm32f4.hex ./targets/stm32f4

# Build multicore test for RP2040 (Pico 1)
test-multicore:
	@mkdir -p build
	$(TINYGO) build -target=pico -size=short -o build/multicore-test-rp2040.uf2 ./test/multicore
	@echo "Built: build/multicore-test-rp2040.uf2"
	@echo "Flash to Pico: cp build/multicore-test-rp2040.uf2 /media/\$$USER/RPI-RP2/"

# Build multicore test for RP2350 (Pico 2)
test-multicore-pico2:
	@mkdir -p build
	$(TINYGO) build -target=pico2 -size=short -o build/multicore-test-rp2350.uf2 ./test/multicore
	@echo "Built: build/multicore-test-rp2350.uf2"
	@echo "Flash to Pico 2: cp build/multicore-test-rp2350.uf2 /media/\$$USER/RPI-RP2/"

# Run tests
test:
	go test ./protocol/... ./core/...

# Clean build artifacts
clean:
	rm -rf build/

# Create build directory
build:
	mkdir -p build