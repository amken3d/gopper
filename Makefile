# Gopper Build System

.PHONY: all clean test rp2040 stm32f4

TINYGO = tinygo

# Default target
all: rp2040

# Build for RP2040 (Raspberry Pi Pico)
rp2040:
	$(TINYGO) build -target=pico -size=short -o build/gopper-rp2040.uf2 ./targets/rp2040

# Build for STM32F4
stm32f4:
	$(TINYGO) build -target=nucleo-f446re -size=short -o build/gopper-stm32f4.hex ./targets/stm32f4

# Run tests (protocol package only - core tests TODO)
test:
	go test -v ./protocol/...

# Clean build artifacts
clean:
	rm -rf build/

# Create build directory
build:
	mkdir -p build