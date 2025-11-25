module gopper

go 1.24.0

toolchain go1.24.7

require (
	github.com/tarm/serial v0.0.0-20180830185346-98f6abe2eb07
	github.com/tinygo-org/pio v0.2.0
	tinygo.org/x/drivers v0.33.0
)

require (
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	golang.org/x/sys v0.38.0 // indirect
)

// Use fork with updated PIO assembler API
replace github.com/tinygo-org/pio => github.com/amken3d/pio v0.1.0
