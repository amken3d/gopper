// Gopper WebSerial UI Application
// Uses TinyGo Wasm for Klipper protocol handling

class GopperUI {
    constructor() {
        this.port = null;
        this.reader = null;
        this.writer = null;
        this.wasmReady = false;
        this.receiveBuffer = new Uint8Array(0);
        this.sequence = 0x10; // Start with MESSAGE_DEST

        // MCU parameters
        this.clockFreq = 12000000; // Default 12MHz, updated from identify response

        // Command dictionary - maps command ID to name and direction
        this.commandDict = {
            0: { name: 'identify_response', dir: 'mcu→host', desc: 'MCU identification data' },
            1: { name: 'identify', dir: 'host→mcu', desc: 'Request identification' },
            2: { name: 'get_uptime', dir: 'host→mcu', desc: 'Request system uptime' },
            3: { name: 'get_clock', dir: 'host→mcu', desc: 'Request current clock' },
            4: { name: 'get_config', dir: 'host→mcu', desc: 'Request configuration' },
            5: { name: 'config_reset', dir: 'host→mcu', desc: 'Reset configuration' },
            6: { name: 'finalize_config', dir: 'host→mcu', desc: 'Finalize configuration' },
            7: { name: 'allocate_oids', dir: 'host→mcu', desc: 'Allocate object IDs' },
            8: { name: 'emergency_stop', dir: 'host→mcu', desc: 'Emergency stop' },
            9: { name: 'reset', dir: 'host→mcu', desc: 'Reset MCU' },
            10: { name: 'debug_read', dir: 'host→mcu', desc: 'Read debug value' },
            11: { name: 'debug_result', dir: 'mcu→host', desc: 'Debug read result' },
            12: { name: 'clock', dir: 'mcu→host', desc: 'Clock value response' },
            13: { name: 'uptime', dir: 'mcu→host', desc: 'Uptime response' },
            14: { name: 'config', dir: 'mcu→host', desc: 'Configuration response' },
        };

        this.initWasm();
        this.setupEventListeners();
    }

    async initWasm() {
        try {
            const go = new Go();
            const result = await WebAssembly.instantiateStreaming(
                fetch('gopper.wasm'),
                go.importObject
            );
            go.run(result.instance);

            // Wait for gopperWasm to be available
            await new Promise((resolve) => {
                const checkWasm = () => {
                    if (window.gopperWasm) {
                        this.wasmReady = true;
                        this.updateStatus('wasm-status', 'Ready', 'connected');
                        this.log('Wasm module loaded successfully', 'info');
                        this.log(`Gopper version: ${window.gopperWasm.version}`, 'info');
                        resolve();
                    } else {
                        setTimeout(checkWasm, 100);
                    }
                };
                checkWasm();
            });
        } catch (error) {
            this.log(`Wasm load error: ${error.message}`, 'error');
            this.updateStatus('wasm-status', 'Failed', 'disconnected');
        }
    }

    setupEventListeners() {
        // Connection buttons
        document.getElementById('connect-btn').addEventListener('click', () => this.connect());
        document.getElementById('disconnect-btn').addEventListener('click', () => this.disconnect());

        // Command buttons
        document.getElementById('cmd-identify').addEventListener('click', () => this.sendIdentify());
        document.getElementById('cmd-uptime').addEventListener('click', () => this.sendGetUptime());
        document.getElementById('cmd-clock').addEventListener('click', () => this.sendGetClock());
        document.getElementById('cmd-config').addEventListener('click', () => this.sendGetConfig());
        document.getElementById('send-custom').addEventListener('click', () => this.sendCustomCommand());

        // Test buttons
        document.getElementById('vlq-encode').addEventListener('click', () => this.testVLQEncode());
        document.getElementById('vlq-decode').addEventListener('click', () => this.testVLQDecode());

        // Console clear
        document.getElementById('clear-console').addEventListener('click', () => this.clearConsole());
    }

    async connect() {
        if (!('serial' in navigator)) {
            this.log('WebSerial not supported in this browser. Use Chrome/Edge.', 'error');
            return;
        }

        if (!this.wasmReady) {
            this.log('Wasm module not ready yet', 'error');
            return;
        }

        try {
            // Request serial port
            this.port = await navigator.serial.requestPort();

            // Open port with standard Klipper settings (250000 baud for UART)
            // Note: RP2040 uses USB CDC, but WebSerial still needs baud rate
            await this.port.open({ baudRate: 250000 });

            this.log('Serial port opened', 'info');
            document.getElementById('port-info').style.display = 'block';
            document.getElementById('port-details').textContent =
                `${this.port.getInfo().usbVendorId}:${this.port.getInfo().usbProductId}`;

            this.updateStatus('connection-status', 'Connected', 'connected');
            document.getElementById('connect-btn').disabled = true;
            document.getElementById('disconnect-btn').disabled = false;

            // Enable command buttons
            this.enableCommandButtons(true);

            // Start reading
            this.startReading();

        } catch (error) {
            this.log(`Connection error: ${error.message}`, 'error');
        }
    }

    async disconnect() {
        if (this.reader) {
            await this.reader.cancel();
            this.reader = null;
        }

        if (this.port) {
            await this.port.close();
            this.port = null;
        }

        this.updateStatus('connection-status', 'Disconnected', 'disconnected');
        this.updateStatus('mcu-status', 'Unknown', '');
        document.getElementById('connect-btn').disabled = false;
        document.getElementById('disconnect-btn').disabled = true;
        document.getElementById('port-info').style.display = 'none';

        this.enableCommandButtons(false);
        this.log('Disconnected', 'info');
    }

    async startReading() {
        this.reader = this.port.readable.getReader();

        try {
            while (true) {
                const { value, done } = await this.reader.read();
                if (done) {
                    this.log('Reader closed', 'info');
                    break;
                }

                // Process received data
                this.handleReceivedData(value);
            }
        } catch (error) {
            this.log(`Read error: ${error.message}`, 'error');
        } finally {
            this.reader.releaseLock();
        }
    }

    handleReceivedData(data) {
        // Convert to hex for logging
        const hexData = Array.from(data)
            .map(b => b.toString(16).padStart(2, '0'))
            .join(' ');

        this.log(`← RX: ${hexData}`, 'recv');

        // Append to receive buffer
        const newBuffer = new Uint8Array(this.receiveBuffer.length + data.length);
        newBuffer.set(this.receiveBuffer);
        newBuffer.set(data, this.receiveBuffer.length);
        this.receiveBuffer = newBuffer;

        // Try to parse complete messages from buffer
        this.parseMessagesFromBuffer();
    }

    parseMessagesFromBuffer() {
        // Look for complete messages (ending with 0x7e sync byte)
        while (this.receiveBuffer.length >= 5) {
            // Find sync byte
            const syncIndex = this.receiveBuffer.indexOf(0x7e);
            if (syncIndex === -1) break;

            // Extract the message up to and including sync
            const msgBytes = this.receiveBuffer.slice(0, syncIndex + 1);

            // Check if we have a complete message
            if (msgBytes.length < 5) {
                // Not enough bytes, wait for more
                break;
            }

            const msgLen = msgBytes[0];
            if (msgBytes.length < msgLen) {
                // Incomplete message, wait for more data
                break;
            }

            // We have a complete message, decode it
            const msgHex = Array.from(msgBytes.slice(0, msgLen))
                .map(b => b.toString(16).padStart(2, '0'))
                .join('');

            this.decodeAndHandleMessage(msgHex);

            // Remove processed message from buffer
            this.receiveBuffer = this.receiveBuffer.slice(syncIndex + 1);
        }
    }

    decodeAndHandleMessage(msgHex) {
        if (!this.wasmReady) return;

        try {
            const decoded = window.gopperWasm.decodeMessage(msgHex);

            if (decoded.error) {
                this.log(`  ⚠ Decode error: ${decoded.error}`, 'error');
                return;
            }

            // Get command info
            const cmdInfo = this.commandDict[decoded.cmdID] || {
                name: `cmd_${decoded.cmdID}`,
                dir: 'unknown',
                desc: 'Unknown command'
            };

            // Build decoded message string
            let msg = `  ◀ [seq=${decoded.sequence}] ${cmdInfo.name} (ID ${decoded.cmdID})`;

            // Add parameters if present
            if (decoded.params && decoded.params.length > 0) {
                const paramStr = decoded.params.map(p => p.value).join(', ');
                msg += ` [${paramStr}]`;
            }

            msg += ` ${decoded.crcValid ? '✓' : '✗ CRC BAD'}`;
            this.log(msg, decoded.crcValid ? 'info' : 'error');

            // Extract payload bytes from the message for handler
            // Message format: len + seq + payload + crc(2) + sync
            const msgBytes = this.hexToBytes(msgHex);
            const payloadBytes = msgBytes.slice(2, msgBytes[0] - 3);

            // Skip command ID VLQ to get just the data
            if (payloadBytes.length > 0) {
                const cmdIdVlq = this.decodeVLQFromBytes(payloadBytes);
                const dataBytes = payloadBytes.slice(cmdIdVlq.consumed);

                // Call the appropriate handler
                this.handleCommand(decoded.cmdID, dataBytes);
            } else {
                // Empty payload (ACK)
                this.handleCommand(decoded.cmdID, new Uint8Array(0));
            }

        } catch (error) {
            this.log(`  Decode exception: ${error.message}`, 'error');
        }
    }

    handleCommand(cmdID, data) {
        // data is already a Uint8Array
        switch (cmdID) {
            case 0: // identify_response
                this.handleIdentifyResponse(data);
                break;
            case 13: // uptime
                this.handleUptimeResponse(data);
                break;
            case 12: // clock
                this.handleClockResponse(data);
                break;
            case 14: // config
                this.handleConfigResponse(data);
                break;
            default:
                // Don't log for ACK-only messages (like seq 0x11 with no payload)
                break;
        }
    }

    handleIdentifyResponse(data) {
        // identify_response: offset=%u data=%*s
        // Note: Can be empty (just an ACK)
        if (data.length === 0) {
            this.updateInfo('info-firmware', 'Gopper');
            this.updateStatus('mcu-status', 'Identified', 'connected');
            this.log(`  MCU acknowledged identify request`, 'info');
            return;
        }

        try {
            // Decode offset
            const offset = this.decodeVLQFromBytes(data);
            const dataBytes = data.slice(offset.consumed);

            // Decode data string (length-prefixed)
            const dataStr = this.decodeVLQStringFromBytes(dataBytes);

            // The data is JSON-like dictionary content
            this.updateInfo('info-firmware', 'Gopper (identified)');
            this.updateStatus('mcu-status', 'Identified', 'connected');
            this.log(`  Firmware data chunk at offset ${offset.value} (${dataStr.length} bytes)`, 'info');
        } catch (err) {
            this.log(`  Error decoding identify_response: ${err.message}`, 'error');
        }
    }

    handleUptimeResponse(data) {
        // uptime: high=%u clock=%u
        // Combined to form 64-bit uptime: (high << 32) | low
        if (data.length === 0) {
            this.log(`  Empty uptime response (ACK only)`, 'info');
            return;
        }

        try {
            const high = this.decodeVLQFromBytes(data);
            const low = this.decodeVLQFromBytes(data.slice(high.consumed));

            // Calculate uptime in ticks (64-bit)
            const uptimeTicks = (high.value * 4294967296) + low.value;

            // Convert to seconds
            const uptimeSeconds = uptimeTicks / this.clockFreq;

            // Format as human-readable
            const uptimeStr = this.formatUptime(uptimeSeconds);

            this.updateInfo('info-uptime', uptimeStr);
            this.log(`  Uptime: ${uptimeStr} (${uptimeTicks.toLocaleString()} ticks)`, 'info');
        } catch (err) {
            this.log(`  Error decoding uptime: ${err.message}`, 'error');
        }
    }

    handleClockResponse(data) {
        // clock: clock=%u
        try {
            const clock = this.decodeVLQFromBytes(data);

            // Convert to seconds
            const seconds = (clock.value / this.clockFreq).toFixed(3);

            const clockStr = `${clock.value.toLocaleString()} ticks (${seconds}s)`;

            this.updateInfo('info-clock', clockStr);
            this.log(`  Current clock: ${clockStr}`, 'info');
        } catch (err) {
            this.log(`  Error decoding clock: ${err.message}`, 'error');
        }
    }

    handleConfigResponse(data) {
        // config: is_config=%c crc=%u is_shutdown=%c move_count=%hu
        try {
            const isConfig = this.decodeVLQFromBytes(data);
            const crc = this.decodeVLQFromBytes(data.slice(isConfig.consumed));
            const isShutdown = this.decodeVLQFromBytes(data.slice(isConfig.consumed + crc.consumed));
            const moveCount = this.decodeVLQFromBytes(data.slice(isConfig.consumed + crc.consumed + isShutdown.consumed));

            // Update UI
            this.updateInfo('info-configured', isConfig.value ? 'Yes' : 'No');
            this.updateInfo('info-config-crc', crc.value === 0 ? 'None' : `0x${crc.value.toString(16)}`);
            this.updateInfo('info-shutdown', isShutdown.value ? 'YES (Emergency Stop!)' : 'No');
            this.updateInfo('info-move-queue', `${moveCount.value} commands`);

            this.log(`  Config: ${isConfig.value ? 'configured' : 'not configured'}, ` +
                     `CRC: ${crc.value}, shutdown: ${isShutdown.value ? 'YES' : 'no'}, ` +
                     `queue: ${moveCount.value}`, 'info');
        } catch (err) {
            this.log(`  Error decoding config: ${err.message}`, 'error');
        }
    }

    // Helper to decode VLQ from byte array
    decodeVLQFromBytes(bytes) {
        let value = 0;
        let shift = 0;
        let i = 0;

        while (i < bytes.length) {
            const byte = bytes[i++];
            value |= (byte & 0x7f) << shift;
            if ((byte & 0x80) === 0) {
                // Sign extend if needed
                if (value & 1) {
                    value = -(value >> 1);
                } else {
                    value = value >> 1;
                }
                return { value, consumed: i };
            }
            shift += 7;
        }
        throw new Error('Incomplete VLQ');
    }

    decodeVLQStringFromBytes(bytes) {
        const length = this.decodeVLQFromBytes(bytes);
        const strBytes = bytes.slice(length.consumed, length.consumed + length.value);
        return new TextDecoder().decode(strBytes);
    }

    formatUptime(seconds) {
        if (seconds < 60) {
            return `${seconds.toFixed(2)}s`;
        } else if (seconds < 3600) {
            const minutes = Math.floor(seconds / 60);
            const secs = (seconds % 60).toFixed(0);
            return `${minutes}m ${secs}s`;
        } else {
            const hours = Math.floor(seconds / 3600);
            const minutes = Math.floor((seconds % 3600) / 60);
            return `${hours}h ${minutes}m`;
        }
    }

    updateInfo(elementId, value) {
        const element = document.getElementById(elementId);
        if (element) {
            element.textContent = value;
        }
    }

    async sendCommand(cmdID, argsHex = '') {
        if (!this.port || !this.wasmReady) {
            this.log('Not connected or Wasm not ready', 'error');
            return;
        }

        try {
            // Encode message using Wasm
            const msgHex = window.gopperWasm.encodeMessage(cmdID, argsHex);

            if (msgHex.startsWith('error:')) {
                this.log(msgHex, 'error');
                return;
            }

            // Convert hex to bytes
            const msgBytes = this.hexToBytes(msgHex);

            this.log(`→ TX: ${msgHex}`, 'send');
            this.decodeAndLogMessage(msgHex, 'TX');

            // Get writer and send
            const writer = this.port.writable.getWriter();
            await writer.write(msgBytes);
            writer.releaseLock();

        } catch (error) {
            this.log(`Send error: ${error.message}`, 'error');
        }
    }

    // Command helpers
    async sendIdentify() {
        // Command ID for identify (you'll need to verify this from your firmware)
        await this.sendCommand(1, '');
    }

    async sendGetUptime() {
        await this.sendCommand(2, '');
    }

    async sendGetClock() {
        await this.sendCommand(3, '');
    }

    async sendGetConfig() {
        await this.sendCommand(4, '');
    }

    async sendCustomCommand() {
        const cmdID = parseInt(document.getElementById('cmd-id').value);
        const argsHex = document.getElementById('cmd-args').value.trim();

        if (isNaN(cmdID)) {
            this.log('Invalid command ID', 'error');
            return;
        }

        await this.sendCommand(cmdID, argsHex);
    }

    // Test functions
    testVLQEncode() {
        const value = parseInt(document.getElementById('vlq-input').value);
        if (isNaN(value)) {
            document.getElementById('vlq-result').textContent = 'Invalid input';
            return;
        }

        const encoded = window.gopperWasm.encodeVLQ(value);
        document.getElementById('vlq-result').textContent = encoded;
        this.log(`VLQ encode: ${value} → ${encoded}`, 'info');
    }

    testVLQDecode() {
        const hexInput = document.getElementById('vlq-decode-input').value.trim();
        const result = window.gopperWasm.decodeVLQ(hexInput);

        if (result.error) {
            document.getElementById('vlq-decode-result').textContent = `Error: ${result.error}`;
        } else {
            document.getElementById('vlq-decode-result').textContent =
                `${result.value} (consumed ${result.consumed} bytes)`;
            this.log(`VLQ decode: ${hexInput} → ${result.value}`, 'info');
        }
    }

    // UI helpers
    log(message, type = 'info') {
        const console = document.getElementById('console');
        const entry = document.createElement('div');
        entry.className = 'log-entry';

        const time = new Date().toLocaleTimeString();
        const timeSpan = document.createElement('span');
        timeSpan.className = 'log-time';
        timeSpan.textContent = `[${time}]`;

        const msgSpan = document.createElement('span');
        msgSpan.className = `log-${type}`;
        msgSpan.textContent = ` ${message}`;

        entry.appendChild(timeSpan);
        entry.appendChild(msgSpan);
        console.appendChild(entry);

        // Auto-scroll to bottom
        console.scrollTop = console.scrollHeight;
    }

    clearConsole() {
        document.getElementById('console').innerHTML = '';
    }

    updateStatus(elementId, text, className) {
        const element = document.getElementById(elementId);
        element.textContent = text;
        element.className = 'status-value';
        if (className) {
            element.classList.add(className);
        }
    }

    enableCommandButtons(enabled) {
        const buttons = document.querySelectorAll('.btn-command');
        buttons.forEach(btn => btn.disabled = !enabled);
    }

    hexToBytes(hex) {
        const bytes = new Uint8Array(hex.length / 2);
        for (let i = 0; i < bytes.length; i++) {
            bytes[i] = parseInt(hex.substr(i * 2, 2), 16);
        }
        return bytes;
    }

    decodeAndLogMessage(hexStr, direction) {
        if (!this.wasmReady) return;

        try {
            const decoded = window.gopperWasm.decodeMessage(hexStr);

            if (decoded.error) {
                this.log(`  ⚠ Decode error: ${decoded.error}`, 'error');
                return;
            }

            // Get command info
            const cmdInfo = this.commandDict[decoded.cmdID] || {
                name: `cmd_${decoded.cmdID}`,
                dir: 'unknown',
                desc: 'Unknown command'
            };

            // Build decoded message string
            let msg = `  ${direction === 'TX' ? '▶' : '◀'} `;
            msg += `[seq=${decoded.sequence}] `;
            msg += `${cmdInfo.name} (ID ${decoded.cmdID})`;

            // Add parameters if present
            if (decoded.params && decoded.params.length > 0) {
                const paramStr = decoded.params.map(p => p.value).join(', ');
                msg += ` [${paramStr}]`;
            }

            // Add CRC status
            msg += ` ${decoded.crcValid ? '✓' : '✗ CRC BAD'}`;

            // Log with appropriate style
            const logType = decoded.crcValid ? 'info' : 'error';
            this.log(msg, logType);

            // Add description for clarity
            if (decoded.params && decoded.params.length > 0) {
                this.log(`    ${cmdInfo.desc} | params: ${this.formatParams(decoded.params)}`, 'info');
            } else {
                this.log(`    ${cmdInfo.desc}`, 'info');
            }

        } catch (error) {
            this.log(`  Decode exception: ${error.message}`, 'error');
        }
    }

    formatParams(params) {
        if (!params || params.length === 0) return 'none';
        return params.map((p, i) => `p${i}=${p.value} (${p.bytes}B)`).join(', ');
    }
}

// Initialize the app when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    const app = new GopperUI();
    window.gopperApp = app; // Make available in console for debugging
});