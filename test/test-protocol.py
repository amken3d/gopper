#!/usr/bin/env python3
"""
Test script for Gopper firmware protocol communication
Tests basic Klipper protocol commands without full Klipper installation
"""

import serial
import struct
import time
import sys

# Klipper protocol constants
MESSAGE_SYNC = 0x7E
MESSAGE_DEST = 0x10
MESSAGE_SEQ_MASK = 0x0F

def crc16(data):
    """Calculate CRC16 for Klipper protocol"""
    crc = 0xFFFF
    for b in data:
        b = b ^ (crc & 0xFF)
        b = b ^ ((b << 4) & 0xFF)
        crc = ((b << 8) | (crc >> 8)) ^ (b >> 4) ^ (b << 3)
        crc &= 0xFFFF
    return crc

def encode_vlq(value):
    """Encode integer as Variable Length Quantity"""
    result = []
    sv = value if value >= 0 else -value

    # Check ranges and output bytes
    if not (-(1 << 26) <= value < (3 << 26)):
        result.append(((value >> 28) & 0x7F) | 0x80)
    if not (-(1 << 19) <= value < (3 << 19)):
        result.append(((value >> 21) & 0x7F) | 0x80)
    if not (-(1 << 12) <= value < (3 << 12)):
        result.append(((value >> 14) & 0x7F) | 0x80)
    if not (-(1 << 5) <= value < (3 << 5)):
        result.append(((value >> 7) & 0x7F) | 0x80)
    result.append(value & 0x7F)

    return bytes(result)

def decode_vlq(data):
    """Decode Variable Length Quantity"""
    c = data[0]
    v = c & 0x7F
    pos = 1

    # Sign extension
    if (c & 0x60) == 0x60:
        v |= (~0x1F & 0xFFFFFFFF)

    # Read continuation bytes
    while c & 0x80:
        c = data[pos]
        pos += 1
        v = (v << 7) | (c & 0x7F)

    # Convert to signed
    if v & 0x80000000:
        v = -(v ^ 0xFFFFFFFF) - 1

    return v, pos

def build_message(seq, command_id, params=b''):
    """Build a Klipper protocol message"""
    # Encode command ID
    frame = encode_vlq(command_id) + params

    # Build header
    length = 2 + len(frame) + 3  # header + frame + trailer
    header = bytes([length, MESSAGE_DEST | (seq & MESSAGE_SEQ_MASK)])

    # Calculate CRC
    msg = header + frame
    crc = crc16(msg)

    # Add trailer
    msg += bytes([
        (crc >> 8) & 0xFF,
        crc & 0xFF,
        MESSAGE_SYNC
    ])

    return msg

def parse_message(data):
    """Parse a received Klipper message"""
    if len(data) < 5:
        return None

    length = data[0]
    seq = data[1]

    if len(data) < length:
        return None

    # Verify CRC
    msg_crc = (data[length-3] << 8) | data[length-2]
    calc_crc = crc16(data[:length-3])

    if msg_crc != calc_crc:
        print(f"CRC mismatch: got {msg_crc:04X}, expected {calc_crc:04X}")
        print(f"Received data (hex): {data[:length].hex()}")
        print(f"Frame data: {data[2:length-3].hex()}")
        return None

    # Check sync byte
    if data[length-1] != MESSAGE_SYNC:
        print(f"Missing sync byte")
        return None

    # Extract frame
    frame = data[2:length-3]
    return {'seq': seq, 'frame': frame}

class GopperTester:
    def __init__(self, port, baudrate=250000):
        self.port = port
        self.baudrate = baudrate
        self.ser = None
        self.seq = MESSAGE_DEST
        self.command_map = {}

    def connect(self):
        """Connect to the serial port"""
        print(f"Connecting to {self.port} at {self.baudrate} baud...")
        self.ser = serial.Serial(self.port, self.baudrate, timeout=2)
        time.sleep(2)  # Wait for reset
        print("Connected!")

    def disconnect(self):
        """Disconnect from serial port"""
        if self.ser:
            self.ser.close()

    def send_command(self, cmd_id, params=b''):
        """Send a command and return response"""
        msg = build_message(self.seq & MESSAGE_SEQ_MASK, cmd_id, params)
        print(f"Sending command {cmd_id}: {msg.hex()}")
        self.ser.write(msg)

        # Increment sequence
        self.seq = ((self.seq + 1) & MESSAGE_SEQ_MASK) | MESSAGE_DEST

        # Read response
        response = self.read_response()
        return response

    def read_response(self, timeout=2):
        """Read and parse response"""
        self.ser.timeout = timeout
        buffer = b''
        start_time = time.time()

        while time.time() - start_time < timeout:
            if self.ser.in_waiting:
                buffer += self.ser.read(self.ser.in_waiting)

            # Try to parse messages
            while len(buffer) >= 5:
                msg = parse_message(buffer)
                if msg:
                    length = buffer[0]
                    buffer = buffer[length:]

                    if len(msg['frame']) > 0:
                        # Decode command ID
                        cmd_id, pos = decode_vlq(msg['frame'])
                        payload = msg['frame'][pos:]
                        print(f"Received: cmd_id={cmd_id}, payload={payload.hex()}")
                        return {'cmd_id': cmd_id, 'payload': payload}
                else:
                    # Look for sync byte
                    sync_pos = buffer.find(bytes([MESSAGE_SYNC]))
                    if sync_pos >= 0:
                        buffer = buffer[sync_pos+1:]
                    else:
                        break

            time.sleep(0.01)

        return None

    def get_dictionary(self):
        """Retrieve the full data dictionary"""
        print("\n=== Retrieving Data Dictionary ===")
        dictionary = b''
        offset = 0
        chunk_size = 40

        # Command ID 1 is 'identify' (ID 0 is identify_response)
        while True:
            params = encode_vlq(offset) + encode_vlq(chunk_size)
            response = self.send_command(1, params)

            if not response:
                break

            # Decode response (identify_response)
            payload = response['payload']
            if len(payload) < 2:
                break

            # Decode offset and data
            resp_offset, pos = decode_vlq(payload)
            data_len, pos2 = decode_vlq(payload[pos:])
            data = payload[pos+pos2:pos+pos2+data_len]

            if len(data) == 0:
                break

            dictionary += data
            offset += len(data)

            if len(data) < chunk_size:
                break

        return dictionary.decode('utf-8', errors='ignore')

    def test_basic_commands(self):
        """Test basic protocol commands"""
        print("\n=== Testing Basic Commands ===")

        # Test get_clock (cmd_id 3 after reordering)
        print("\nTesting get_clock...")
        response = self.send_command(3, b'')
        if response:
            clock, _ = decode_vlq(response['payload'])
            print(f"Clock value: {clock}")

        # Test get_uptime (cmd_id 2 after reordering)
        print("\nTesting get_uptime...")
        response = self.send_command(2, b'')
        if response:
            high, pos = decode_vlq(response['payload'])
            low, _ = decode_vlq(response['payload'][pos:])
            uptime = (high << 32) | low
            print(f"Uptime: {uptime} ticks ({uptime/1000000:.2f} seconds)")

def main():
    if len(sys.argv) < 2:
        print("Usage: python3 test-protocol.py <serial_port>")
        print("Example: python3 test-protocol.py /dev/ttyACM0")
        sys.exit(1)

    port = sys.argv[1]

    tester = GopperTester(port)

    try:
        tester.connect()

        # Get data dictionary
        dictionary = tester.get_dictionary()
        print("\n=== Data Dictionary ===")
        print(dictionary)

        # Test basic commands
        tester.test_basic_commands()

        print("\n=== Test Complete ===")

    except KeyboardInterrupt:
        print("\nTest interrupted by user")
    except Exception as e:
        print(f"\nError: {e}")
        import traceback
        traceback.print_exc()
    finally:
        tester.disconnect()

if __name__ == '__main__':
    main()