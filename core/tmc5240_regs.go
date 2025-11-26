package core

// TMC5240 Register Definitions
// Based on TMC5240 datasheet Rev. 1.09 / 2021-06-02
// Trinamic Motion Control GmbH & Co. KG

// TMC5240 Register Addresses
const (
	// General Configuration Registers (0x00-0x0F)
	TMC5240_GCONF      = 0x00 // Global configuration flags
	TMC5240_GSTAT      = 0x01 // Global status flags
	TMC5240_IFCNT      = 0x02 // Interface transmission counter
	TMC5240_SLAVECONF  = 0x03 // Slave configuration
	TMC5240_IOIN       = 0x04 // Reads the state of all input pins
	TMC5240_OUTPUT     = 0x05 // Output pin control
	TMC5240_X_COMPARE  = 0x06 // Position comparison register
	TMC5240_FACTORY_CONF = 0x08 // Factory configuration

	// Velocity Dependent Driver Feature Control (0x10-0x1F)
	TMC5240_SHORT_CONF = 0x09 // Short detector configuration
	TMC5240_DRV_CONF   = 0x0A // Driver configuration
	TMC5240_GLOBAL_SCALER = 0x0B // Global current scaler

	// Motor Driver Registers (0x6C-0x7F)
	TMC5240_IHOLD_IRUN = 0x10 // Driver current control
	TMC5240_TPOWERDOWN = 0x11 // Delay after standstill
	TMC5240_TSTEP      = 0x12 // Measured time between two steps (read only)
	TMC5240_TPWMTHRS   = 0x13 // Upper velocity for StealthChop
	TMC5240_TCOOLTHRS  = 0x14 // Lower threshold velocity for CoolStep
	TMC5240_THIGH      = 0x15 // High velocity threshold

	// Ramp Generator Motion Control (0x20-0x2D)
	TMC5240_RAMPMODE   = 0x20 // Ramp mode (0=positioning, 1=velocity positive, 2=velocity negative, 3=hold)
	TMC5240_XACTUAL    = 0x21 // Actual motor position (signed, read/write)
	TMC5240_VACTUAL    = 0x22 // Actual motor velocity (read only)
	TMC5240_VSTART     = 0x23 // Motor start velocity
	TMC5240_A1         = 0x24 // First acceleration between VSTART and V1
	TMC5240_V1         = 0x25 // First acceleration/deceleration phase threshold velocity
	TMC5240_AMAX       = 0x26 // Second acceleration between V1 and VMAX
	TMC5240_VMAX       = 0x27 // Maximum velocity (motion ramp)
	TMC5240_DMAX       = 0x28 // Deceleration between VMAX and V1
	TMC5240_D1         = 0x2A // Deceleration between V1 and VSTOP
	TMC5240_VSTOP      = 0x2B // Motor stop velocity
	TMC5240_TZEROWAIT  = 0x2C // Waiting time after ramping down to zero velocity
	TMC5240_XTARGET    = 0x2D // Target position (signed)

	// Ramp Generator Status (0x33-0x37)
	TMC5240_VDCMIN     = 0x33 // Automatic commutation dcStep minimum velocity
	TMC5240_SW_MODE    = 0x34 // Switch mode configuration
	TMC5240_RAMP_STAT  = 0x35 // Ramp and reference switch status
	TMC5240_XLATCH     = 0x36 // Latched position for next interrupt

	// Encoder Registers (0x38-0x3C)
	TMC5240_ENCMODE    = 0x38 // Encoder configuration and use of N channel
	TMC5240_X_ENC      = 0x39 // Actual encoder position
	TMC5240_ENC_CONST  = 0x3A // Accumulation constant
	TMC5240_ENC_STATUS = 0x3B // Encoder status information
	TMC5240_ENC_LATCH  = 0x3C // Encoder latch position

	// Motor Driver Registers (0x6C-0x7F)
	TMC5240_MSLUT0     = 0x60 // Microstep table entry 0
	TMC5240_MSLUT1     = 0x61 // Microstep table entry 1
	TMC5240_MSLUT2     = 0x62 // Microstep table entry 2
	TMC5240_MSLUT3     = 0x63 // Microstep table entry 3
	TMC5240_MSLUT4     = 0x64 // Microstep table entry 4
	TMC5240_MSLUT5     = 0x65 // Microstep table entry 5
	TMC5240_MSLUT6     = 0x66 // Microstep table entry 6
	TMC5240_MSLUT7     = 0x67 // Microstep table entry 7
	TMC5240_MSLUTSEL   = 0x68 // Microstep table selector
	TMC5240_MSLUTSTART = 0x69 // Microstep table start offset

	TMC5240_MSCNT      = 0x6A // Microstep counter (read only)
	TMC5240_MSCURACT   = 0x6B // Actual microstep current (read only)
	TMC5240_CHOPCONF   = 0x6C // Chopper configuration
	TMC5240_COOLCONF   = 0x6D // CoolStep configuration
	TMC5240_DCCTRL     = 0x6E // dcStep automatic commutation
	TMC5240_DRV_STATUS = 0x6F // Driver status flags and current level read back
	TMC5240_PWMCONF    = 0x70 // StealthChop PWM configuration
	TMC5240_PWM_SCALE  = 0x71 // PWM scale value (read only)
	TMC5240_PWM_AUTO   = 0x72 // PWM automatic scale (read only)
	TMC5240_SG4_THRS   = 0x74 // StallGuard4 threshold
	TMC5240_SG4_RESULT = 0x75 // StallGuard4 result (read only)
	TMC5240_SG4_IND    = 0x76 // StallGuard4 current indicator (read only)
)

// TMC5240 GCONF Register Bit Definitions
const (
	TMC5240_GCONF_RECALIBRATE       = 1 << 0  // Zero crossing recalibration
	TMC5240_GCONF_FASTSTANDSTILL    = 1 << 1  // Timeout for step frequency detection
	TMC5240_GCONF_EN_PWM_MODE       = 1 << 2  // Enable StealthChop PWM mode
	TMC5240_GCONF_MULTISTEP_FILT    = 1 << 3  // Enable step input filtering
	TMC5240_GCONF_SHAFT             = 1 << 4  // Inverse motor direction
	TMC5240_GCONF_DIAG0_ERROR       = 1 << 5  // Enable DIAG0 active on driver errors
	TMC5240_GCONF_DIAG0_OTPW        = 1 << 6  // Enable DIAG0 active on overtemperature
	TMC5240_GCONF_DIAG0_STALL       = 1 << 7  // Enable DIAG0 active on stall
	TMC5240_GCONF_DIAG1_STALL       = 1 << 8  // Enable DIAG1 active on stall
	TMC5240_GCONF_DIAG1_INDEX       = 1 << 9  // Enable DIAG1 active on index
	TMC5240_GCONF_DIAG1_ONSTATE     = 1 << 10 // Enable DIAG1 active when chopper on
	TMC5240_GCONF_DIAG1_STEPS_SKIPPED = 1 << 11 // Enable DIAG1 active when steps skipped
	TMC5240_GCONF_DIAG0_INT_PUSHPULL = 1 << 12 // DIAG0 push-pull output
	TMC5240_GCONF_DIAG1_POSCOMP_PUSHPULL = 1 << 13 // DIAG1 push-pull output
	TMC5240_GCONF_SMALL_HYSTERESIS  = 1 << 14 // Small hysteresis
	TMC5240_GCONF_STOP_ENABLE       = 1 << 15 // Emergency stop
	TMC5240_GCONF_DIRECT_MODE       = 1 << 16 // Motor coil currents direct control
)

// TMC5240 RAMP_STAT Register Bit Definitions
const (
	TMC5240_RAMP_STAT_STATUS_STOP_L    = 1 << 0  // Left stop switch status
	TMC5240_RAMP_STAT_STATUS_STOP_R    = 1 << 1  // Right stop switch status
	TMC5240_RAMP_STAT_STATUS_LATCH_L   = 1 << 2  // Left stop latch
	TMC5240_RAMP_STAT_STATUS_LATCH_R   = 1 << 3  // Right stop latch
	TMC5240_RAMP_STAT_EVENT_STOP_L     = 1 << 4  // Left stop event
	TMC5240_RAMP_STAT_EVENT_STOP_R     = 1 << 5  // Right stop event
	TMC5240_RAMP_STAT_EVENT_STOP_SG    = 1 << 6  // StallGuard2 stop event
	TMC5240_RAMP_STAT_EVENT_POS_REACHED = 1 << 7 // Target position reached
	TMC5240_RAMP_STAT_VELOCITY_REACHED = 1 << 8  // Target velocity reached
	TMC5240_RAMP_STAT_POSITION_REACHED = 1 << 9  // Target position reached
	TMC5240_RAMP_STAT_VZERO            = 1 << 10 // Velocity is 0
	TMC5240_RAMP_STAT_T_ZEROWAIT_ACTIVE = 1 << 11 // TZEROWAIT active
	TMC5240_RAMP_STAT_SECOND_MOVE      = 1 << 12 // Second move in progress
	TMC5240_RAMP_STAT_STATUS_SG        = 1 << 13 // StallGuard status
)

// TMC5240 DRV_STATUS Register Bit Definitions
const (
	TMC5240_DRV_STATUS_SG_RESULT = 0x3FF     // StallGuard result mask (bits 0-9)
	TMC5240_DRV_STATUS_S2VSA     = 1 << 12   // Short to supply indicator phase A
	TMC5240_DRV_STATUS_S2VSB     = 1 << 13   // Short to supply indicator phase B
	TMC5240_DRV_STATUS_STEALTH   = 1 << 14   // StealthChop indicator
	TMC5240_DRV_STATUS_FSACTIVE  = 1 << 15   // Full step active indicator
	TMC5240_DRV_STATUS_CS_ACTUAL = 0x1F << 16 // Actual current control scaling
	TMC5240_DRV_STATUS_STALLGUARD = 1 << 24  // StallGuard status
	TMC5240_DRV_STATUS_OT        = 1 << 25   // Overtemperature flag
	TMC5240_DRV_STATUS_OTPW      = 1 << 26   // Overtemperature pre-warning
	TMC5240_DRV_STATUS_S2GA      = 1 << 27   // Short to ground indicator phase A
	TMC5240_DRV_STATUS_S2GB      = 1 << 28   // Short to ground indicator phase B
	TMC5240_DRV_STATUS_OLA       = 1 << 29   // Open load indicator phase A
	TMC5240_DRV_STATUS_OLB       = 1 << 30   // Open load indicator phase B
	TMC5240_DRV_STATUS_STST      = 1 << 31   // Standstill indicator
)

// TMC5240 Ramp Modes
const (
	TMC5240_MODE_POSITION  = 0 // Positioning mode (uses XTARGET)
	TMC5240_MODE_VELOCITY_POS = 1 // Velocity mode (positive VMAX)
	TMC5240_MODE_VELOCITY_NEG = 2 // Velocity mode (negative VMAX)
	TMC5240_MODE_HOLD      = 3 // Hold mode (velocity = 0)
)

// TMC5240 SPI Access
const (
	TMC5240_WRITE_BIT = 0x80 // Write access bit (set bit 7)
	TMC5240_READ_BIT  = 0x00 // Read access (bit 7 = 0)
)

// Default configuration values
const (
	// Clock frequency (internal oscillator)
	TMC5240_FCLK = 12000000 // 12 MHz

	// Default current settings (example values - adjust for your motor)
	TMC5240_IHOLD_DEFAULT = 10 // Standstill current (0-31)
	TMC5240_IRUN_DEFAULT  = 31 // Run current (0-31)
	TMC5240_IHOLDDELAY_DEFAULT = 10 // Hold delay (0-15)

	// Default chopper settings for silent operation
	TMC5240_CHOPCONF_DEFAULT = 0x000100C3 // Example: TOFF=3, HSTRT=4, HEND=1, TBL=2

	// Default PWM settings for StealthChop
	TMC5240_PWMCONF_DEFAULT = 0xC10D0024 // Example: PWM_FREQ=2, PWM_AUTOSCALE=1
)
