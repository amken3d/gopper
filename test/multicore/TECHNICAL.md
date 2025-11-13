# TinyGo Multicore Technical Details

This document explains how multicore works on RP2040/RP2350 in TinyGo and the different communication methods available.

## RP2040 Multicore Architecture

The RP2040 features:
- **2x ARM Cortex-M0+ cores** at 133MHz
- **Symmetric multiprocessing** - both cores have equal access to all resources
- **Shared memory** - 264KB SRAM accessible by both cores
- **Hardware FIFO** - Two 32-bit FIFOs for inter-core messaging
- **32 Hardware Spinlocks** - For mutual exclusion
- **Cross-core interrupts** - Via the FIFO

## TinyGo Runtime Multicore Support

TinyGo's runtime for RP2040 (as seen in `runtime/runtime_rp2040.go`) implements:

### 1. Core Startup

```go
machine.Core1.Start(functionToRun)
```

This uses the RP2040 hardware startup sequence:
1. Sends magic sequence via FIFO
2. Passes interrupt vector, stack pointer, and entry point
3. Core 1 starts and runs the scheduler

### 2. Scheduler Integration

Both cores run the TinyGo scheduler independently:
- Goroutines can run on either core
- The scheduler uses hardware spinlock #21 for synchronization
- `runtime.GOMAXPROCS()` returns 2 on RP2040

### 3. Garbage Collection Synchronization

The GC uses hardware FIFO to pause all cores during stop-the-world:
1. GC core sends interrupt via FIFO (writes value `1`)
2. Other core handles interrupt in `gcInterruptHandler()`
3. Both cores scan their stacks
4. GC core signals completion via `gcSignalCore()`

### 4. Hardware Spinlocks

TinyGo reserves several spinlocks for runtime use:
- **Spinlock 20**: Print lock (for `println`)
- **Spinlock 21**: Scheduler lock
- **Spinlock 22**: Atomics lock (for non-atomic operations)
- **Spinlock 23**: Futex lock

This leaves spinlocks 0-19 and 24-31 for user code.

## Inter-Core Communication Methods

There are several ways to communicate between cores in TinyGo on RP2040:

### Method 1: Channels (Recommended)

**Pros:**
- Idiomatic Go
- Type-safe
- Blocking/non-blocking options
- Integrated with scheduler

**Cons:**
- Some overhead due to scheduler interaction
- Uses spinlocks internally

**Example:**
```go
ch := make(chan int)

machine.Core1.Start(func() {
    for {
        val := <-ch
        println("Core 1 received:", val)
    }
})

ch <- 42
```

### Method 2: Atomic Variables

**Pros:**
- Very fast
- Lock-free
- Low overhead

**Cons:**
- Limited to numeric types
- No blocking (must poll)
- No ordering guarantees beyond atomicity

**Example:**
```go
var counter atomic.Uint32

machine.Core1.Start(func() {
    for {
        counter.Add(1)
    }
})

value := counter.Load()
```

This is what the main multicore test uses (see `main.go`).

### Method 3: Volatile Registers

**Pros:**
- Very fast
- No atomic overhead
- Direct memory access

**Cons:**
- Not thread-safe (must use with care)
- No compiler reordering protection
- Requires manual synchronization

**Example:**
```go
var message volatile.Register32

machine.Core1.Start(func() {
    for {
        msg := message.Get()
        if msg != 0 {
            // Process message
            message.Set(0) // Clear
        }
    }
})

message.Set(42)
```

### Method 4: Hardware FIFO

**Pros:**
- Hardware-accelerated
- Built-in blocking/waiting (WFE instruction)
- Very efficient

**Cons:**
- TinyGo runtime uses it for GC (conflict risk)
- Limited to 32-bit values
- Only 8-entry depth

**Example:**
See `fifo_example.go` for a complete example.

**⚠️ WARNING**: The TinyGo runtime uses the FIFO for GC synchronization. Using the FIFO directly in user code may interfere with garbage collection. Only use FIFO communication if you understand the implications.

### Method 5: Hardware Spinlocks

**Pros:**
- Hardware-accelerated mutual exclusion
- Very fast
- No busy-wait CPU overhead (uses WFE)

**Cons:**
- Manual lock/unlock required
- Limited to 32 spinlocks (some reserved by runtime)
- Risk of deadlock if not careful

**Example:**
```go
import "device/rp"

// Use spinlock 0 (not reserved by runtime)
func lockSpinlock0() {
    for rp.SIO.SPINLOCK0.Get() == 0 {
        arm.Asm("wfe")
    }
}

func unlockSpinlock0() {
    rp.SIO.SPINLOCK0.Set(0)
    arm.Asm("sev")
}
```

### Method 6: Shared Memory with sync.Mutex

**Pros:**
- Familiar to Go developers
- Type-safe
- Works with any data structure

**Cons:**
- Higher overhead than atomics
- Uses spinlocks internally
- Can block for longer

**Example:**
```go
var (
    sharedData int
    mu sync.Mutex
)

machine.Core1.Start(func() {
    for {
        mu.Lock()
        sharedData++
        mu.Unlock()
    }
})
```

## Comparison Table

| Method | Speed | Safety | Ease of Use | Best For |
|--------|-------|--------|-------------|----------|
| Channels | Medium | High | High | General communication |
| Atomics | Very Fast | High | Medium | Counters, flags |
| Volatile | Very Fast | Low | Low | Memory-mapped I/O |
| FIFO | Very Fast | Medium | Low | Low-level (avoid) |
| Spinlocks | Very Fast | Medium | Low | Mutual exclusion |
| Mutex | Medium | High | High | Protecting shared data |

## Recommendations

### For Gopper Firmware

Based on the Gopper architecture, here are recommendations for multicore usage:

#### Use Case 1: Stepper Pulse Generation
**Recommended:** Dedicated goroutine on Core 1 with atomic variables for coordination

```go
var stepperCommand atomic.Uint32

machine.Core1.Start(func() {
    for {
        cmd := stepperCommand.Load()
        if cmd != 0 {
            generateStepPulse()
            stepperCommand.Store(0)
        }
    }
})
```

#### Use Case 2: Sensor Reading
**Recommended:** Goroutine with channels

```go
sensorData := make(chan SensorReading, 10)

machine.Core1.Start(func() {
    for {
        reading := readSensor()
        sensorData <- reading
    }
})
```

#### Use Case 3: Motion Planning
**Recommended:** Task queue with mutex-protected shared state

```go
var (
    moveQueue []Move
    queueMutex sync.Mutex
)

machine.Core1.Start(func() {
    for {
        queueMutex.Lock()
        if len(moveQueue) > 0 {
            move := moveQueue[0]
            moveQueue = moveQueue[1:]
            queueMutex.Unlock()
            executeMove(move)
        } else {
            queueMutex.Unlock()
            time.Sleep(100 * time.Microsecond)
        }
    }
})
```

## Memory Considerations

### Cache Coherency
RP2040 has small instruction caches (8KB per core) but **no data cache**. This means:
- No cache coherency issues for data
- Atomic operations work correctly
- No need for memory barriers (beyond compiler barriers)

### Memory Access Patterns
Both cores share the same SRAM:
- Contention on same memory bank can slow access
- SRAM is divided into 6 banks
- Try to use different memory regions for different cores

### DMA Considerations
DMA controllers can access memory while cores are running:
- Use volatile for DMA buffers
- Ensure proper synchronization
- Consider cache flushing (not needed on RP2040 as no data cache)

## WFE/SEV Instructions

The ARM `WFE` (Wait For Event) and `SEV` (Send Event) instructions are critical for efficient multicore:

- **WFE**: Puts core to sleep until event received
- **SEV**: Wakes all cores waiting in WFE
- Used by spinlocks and FIFO operations
- Much more efficient than busy-waiting

Example:
```go
import "device/arm"

// Efficient wait instead of busy loop
for !condition() {
    arm.Asm("wfe")
}

// Wake other core
arm.Asm("sev")
```

## Debugging Multicore Code

### Common Issues

1. **Race Conditions**
   - Symptom: Intermittent failures, wrong values
   - Solution: Use atomic operations or mutexes
   - Tool: Go race detector (doesn't work with TinyGo, use careful code review)

2. **Deadlock**
   - Symptom: Both cores stop responding
   - Solution: Ensure lock ordering, use timeouts
   - Debug: LED blink patterns from each core

3. **Starvation**
   - Symptom: One core doing all the work
   - Solution: Balance workload, check scheduler
   - Debug: Use counters to track core activity

4. **Memory Corruption**
   - Symptom: Crashes, panics, wrong values
   - Solution: Use atomic operations, avoid volatile unless needed
   - Debug: Minimize shared state

### Debug Techniques

**LED Patterns:**
```go
// Core 0: Fast blink
// Core 1: Slow blink

machine.Core1.Start(func() {
    led := machine.GP15
    led.Configure(machine.PinConfig{Mode: machine.PinOutput})
    for {
        led.Toggle()
        time.Sleep(1 * time.Second)
    }
})
```

**Counters:**
```go
var (
    core0Counter atomic.Uint32
    core1Counter atomic.Uint32
)

// Periodically print ratio - should be close to 1.0
println("Ratio:", float32(core1Counter.Load())/float32(core0Counter.Load()))
```

**Serial Output:**
```go
// println() is safe to use from both cores (uses spinlock 20)
println("Core", currentCPU(), "event:", eventName)
```

## Performance Tips

1. **Keep critical paths lock-free** - Use atomics instead of mutexes for hot paths
2. **Minimize shared state** - Each core should have mostly independent data
3. **Use the right communication method** - Channels for complex data, atomics for simple values
4. **Batch operations** - Don't send individual items, batch them
5. **Pin goroutines to cores** - TinyGo scheduler may move goroutines between cores

## Further Reading

- [RP2040 Datasheet, Chapter 2.3: Multicore](https://datasheets.raspberrypi.com/rp2040/rp2040-datasheet.pdf)
- [ARM Cortex-M0+ Technical Reference Manual](https://developer.arm.com/documentation/ddi0484/latest/)
- [TinyGo Machine Package](https://tinygo.org/docs/reference/microcontrollers/machine/)
- [Raspberry Pi Pico SDK Documentation](https://www.raspberrypi.com/documentation/pico-sdk/)

## Credits

TinyGo multicore implementation by the TinyGo team:
- https://github.com/tinygo-org/tinygo

Based on Raspberry Pi Pico SDK by Raspberry Pi Ltd.
