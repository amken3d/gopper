# Gopper Firmware Testing Guide

This guide will help you test the Gopper firmware with a Klipper host.

## Communication Method

Gopper uses **USB CDC (Communications Device Class)** as its native communication method. This means:

- ✅ **No UART/serial adapter needed** - Just plug in a USB cable
- ✅ **Automatic enumeration** - Appears as `/dev/ttyACM0` (Linux) or `/dev/tty.usbmodem*` (macOS)
- ✅ **Faster than UART** - 12 Mbps USB full-speed vs 250 kbps UART
- ℹ️ **Baud rate ignored** - USB doesn't use baud rates, but Klipper config still requires it (use 250000)

For detailed USB information and troubleshooting, see **[USB_TESTING.md](USB_TESTING.md)**.

## Prerequisites

- Raspberry Pi Pico or Pico 2 board
- USB cable (USB-A to micro-USB or USB-C depending on your board)
- Linux machine (or macOS/Windows with adjustments)
- Python 3 with `pyserial` package
- Optional: Full Klipper installation

## Quick Test (Without Full Klipper)

### 1. Flash the Firmware

```bash
# Build the firmware (if not already done)
cd ~/repos/gopper
make rp2040          # For Pico 1
# OR
make clean && tinygo build -target=pico2 -o build/gopper-rp2350.uf2 ./targets/rp2040  # For Pico 2

# Flash to Pico:
# 1. Hold BOOTSEL button on Pico
# 2. Connect USB cable
# 3. Release BOOTSEL (Pico mounts as USB drive)
# 4. Copy firmware:

cp build/gopper-rp2040.uf2 /media/$USER/RPI-RP2/
# OR for Pico 2
cp build/gopper-rp2350.uf2 /media/$USER/RPI-RP2/

# The Pico will automatically reboot and unmount
```

### 2. Find the USB Serial Port

After flashing, the Pico will enumerate as a USB CDC device:

```bash
# Linux - should appear as /dev/ttyACM0
ls /dev/ttyACM*

# macOS - should appear as /dev/tty.usbmodem*
ls /dev/tty.usbmodem*

# Verify USB enumeration (Linux)
lsusb | grep -i pico
# Should show: Bus XXX Device XXX: ID 2e8a:XXXX Raspberry Pi

# Check kernel messages
dmesg | tail -20
# Should show: cdc_acm 1-X:1.0: ttyACM0: USB ACM device
```

**Note**: The device appears as `/dev/ttyACM0` because Gopper uses USB CDC, not UART. No baud rate configuration is actually used (USB is always full-speed), but Klipper config still requires a baud rate entry - use `250000` to match Klipper conventions.

### 3. Test with Python Script

```bash
# Install pyserial if needed
pip3 install pyserial

# Make test script executable
chmod +x test/test-protocol.py

# Run the test (replace /dev/ttyACM0 with your port)
python3 test/test-protocol.py /dev/ttyACM0
```

**Expected Output:**
```
Connecting to /dev/ttyACM0 at 250000 baud...
(Note: Baud rate is ignored for USB CDC, but specified for compatibility)
Connected!

=== Retrieving Data Dictionary ===
Sending command 0: ...
Received: cmd_id=1, payload=...

=== Data Dictionary ===
#version gopper-0.1.0
#build_versions go-tinygo
#CLOCK_FREQ=1000000
#MCU=rp2040
#STATS_SUMSQ_BASE=256
identify offset=%u count=%c
get_uptime
get_clock
...

=== Testing Basic Commands ===

Testing get_clock...
Sending command 2: ...
Received: cmd_id=3, payload=...
Clock value: 1234567

Testing get_uptime...
Sending command 1: ...
Received: cmd_id=2, payload=...
Uptime: 1234567 ticks (1.23 seconds)

=== Test Complete ===
```

## Full Klipper Test

### 1. Install Klipper (if not already installed)

```bash
# Clone Klipper
cd ~
git clone https://github.com/Klipper3d/klipper
cd klipper

# Install dependencies
./scripts/install-octopi.sh

# Start Klipper service (or run manually)
sudo systemctl start klipper
```

### 2. Configure Klipper

```bash
# Copy test config
cp ~/repos/gopper/test/klipper-test.cfg ~/printer.cfg

# Edit the serial port in printer.cfg
nano ~/printer.cfg

# Update this line with your actual port:
# serial: /dev/ttyACM0
```

### 3. Start Klipper

```bash
# If using systemd
sudo systemctl restart klipper

# OR run manually for debugging
cd ~/klipper
./klippy/klippy.py ~/printer.cfg -l /tmp/klippy.log

# Watch the log in another terminal
tail -f /tmp/klippy.log
```

### 4. Check Klipper Log

```bash
tail -f /tmp/klippy.log
```

**Expected Log Output:**
```
Starting Klippy...
Args: ['./klippy/klippy.py', '~/printer.cfg', '-l', '/tmp/klippy.log']
MCU 'mcu' config: Klipper version...
Loaded MCU 'mcu' 105 commands (version gopper-0.1.0 build go-tinygo)
MCU 'mcu' computed clock: 1000000
Configured MCU 'mcu' (1024 moves)
Printer is ready
```

## Troubleshooting

### No Serial Port Appears

**Symptoms:** No `/dev/ttyACM*` device after plugging in Pico

**Solutions:**
```bash
# Check if device is detected at all
lsusb | grep -i pico

# Check dmesg for USB events
dmesg | tail -20

# Check permissions
ls -l /dev/ttyACM0
# Should show: crw-rw---- 1 root dialout

# Add your user to dialout group
sudo usermod -a -G dialout $USER
# Then log out and log back in
```

### Permission Denied on Serial Port

```bash
# Temporary fix
sudo chmod 666 /dev/ttyACM0

# Permanent fix - add user to dialout group
sudo usermod -a -G dialout $USER
# Log out and back in
```

### "CRC mismatch" Errors

**Possible Causes:**
- USB connection issues (try different cable/port)
- Firmware not flashed correctly
- USB enumeration failed (check `lsusb` and `dmesg`)
- Cable/connection issues

**Solutions:**
```bash
# Re-flash firmware
# Make sure to build cleanly first
cd ~/repos/gopper
make clean
make rp2040

# Flash again with BOOTSEL button
```

### Python Script Hangs

**Symptoms:** Script connects but no responses

**Debug Steps:**
```bash
# Test raw serial communication
screen /dev/ttyACM0 250000
# OR
minicom -D /dev/ttyACM0 -b 250000

# You should see binary data when commands are sent
# Press Ctrl-A then K to exit screen
# Press Ctrl-A then X to exit minicom
```

### Klipper Says "Unable to connect"

**Check:**
1. Correct serial port in `printer.cfg`
2. Firmware actually running (power cycle Pico)
3. Klipper log for specific errors: `tail -f /tmp/klippy.log`

```bash
# Test with direct connection
python3 -c "import serial; s=serial.Serial('/dev/ttyACM0', 250000, timeout=1); print('Connected'); s.close()"
```

## Monitoring Communication

### Using Wireshark/tcpdump

```bash
# Capture USB serial traffic
sudo tcpdump -i usbmon1 -w capture.pcap

# View in Wireshark
wireshark capture.pcap
```

### Using Serial Monitor

**Note**: Baud rate parameter is ignored for USB CDC but required by these tools.

```bash
# Screen
screen /dev/ttyACM0 250000

# Minicom
minicom -D /dev/ttyACM0 -b 250000

# Python
python3 -m serial.tools.miniterm /dev/ttyACM0 250000
```

### USB-Specific Debugging

See **[USB_TESTING.md](USB_TESTING.md)** for:
- USB enumeration troubleshooting
- USB descriptor information
- USB vs UART comparison
- Advanced USB debugging

## What's Working

After successful tests, you should see:

✅ Firmware identifies itself to Klipper
✅ Data dictionary transmitted correctly
✅ Clock/uptime commands respond
✅ Configuration commands accepted
✅ Emergency stop command works

## What's NOT Working Yet

⚠️ No stepper control (Phase 3)
⚠️ No endstop support
⚠️ No ADC/temperature reading
⚠️ No PWM outputs

## Next Steps

1. **Verify basic communication** - Run test script, check responses
2. **Test with Klipper** - Connect to full Klipper, check logs
3. **Report results** - Share output/logs for debugging
4. **Move to Phase 3** - Add stepper control once communication is verified

## Getting Help

If you encounter issues:

1. Check this troubleshooting section
2. Review `test-protocol.py` output
3. Check `/tmp/klippy.log` if using full Klipper
4. Share error messages and logs

Good luck testing! 🚀