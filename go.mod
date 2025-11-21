module gopper

go 1.25

require (
	github.com/tinygo-org/pio v0.2.0
	tinygo.org/x/drivers v0.33.0
)

require github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect

// Use fork with updated PIO assembler API
replace github.com/tinygo-org/pio => github.com/amken3d/pio v0.1.0
