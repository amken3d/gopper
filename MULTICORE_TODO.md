# Multicore Implementation TODO

## Overview

Plan to leverage RP2040/RP2350 dual-core capabilities using idiomatic Go patterns ("share memory by communicating").

**Status**: Planning Phase
**Target**: Dual-core architecture with Core 0 (Communication) and Core 1 (Motion/Real-time)

---

## Current State Assessment

### Critical Issues Identified
- [ ] ❌ `inputBuffer` and `outputBuffer` have NO synchronization (race conditions)
- [ ] ❌ Timer scheduler uses interrupt disable/restore for entire operations (bottleneck)
- [ ] ❌ `moveCount` is not atomic
- [x] ✅ Command registry already uses proper `RWMutex`
- [x] ✅ Most state flags are already atomic

---

## Proposed Architecture

### Core 0: Communication & Control Core
**Responsibilities:**
- USB I/O and protocol parsing
- Command reception and validation
- Non-real-time command execution (config, status queries)
- Host communication and error reporting
- Dictionary compression and transmission

### Core 1: Motion & Real-Time Core
**Responsibilities:**
- Stepper timer dispatch
- Step pulse generation
- Endstop monitoring
- Real-time motion control loop
- Time-critical operations only

---

## Phase 1: Safety First - Fix Race Conditions (Week 1)

### 1.1 Add Buffer Synchronization
- [ ] Create `core/buffers.go` with thread-safe buffer wrappers
- [ ] Implement `SafeFifoBuffer` with mutex protection
- [ ] Implement `SafeScratchOutput` with mutex protection
- [ ] Update `targets/rp2040/main.go` to use safe buffers
- [ ] Add mutex protection to `inputBuffer`
- [ ] Add mutex protection to `outputBuffer`

### 1.2 Make All Shared State Atomic
- [ ] Convert `moveCount` to `atomic.Uint32` in `core/state.go`
- [ ] Audit all shared state for race conditions
- [ ] Update `FirmwareState` struct with atomic types
- [ ] Add documentation for atomic usage patterns

### 1.3 Testing
- [ ] Run tests with `-race` detector
- [ ] Add unit tests for concurrent buffer access
- [ ] Verify no race conditions in existing code
- [ ] Document synchronization requirements

**Success Criteria**: No race conditions detected, all shared state properly synchronized

---

## Phase 2: Channel-Based Command Pipeline (Week 2-3)

### 2.1 Design Command Queue System
- [ ] Create `core/commandqueue.go`
- [ ] Define `MotionCommand` struct with type and parameters
- [ ] Define `Response` struct for command results
- [ ] Define `Event` struct for endstops/errors
- [ ] Implement `CommandQueue` with buffered channels
  - [ ] `motionCh` channel (Core 0 → Core 1)
  - [ ] `responseCh` channel (Core 1 → Core 0)
  - [ ] `eventCh` channel (Core 1 → Core 0)
- [ ] Implement `SendMotion()` with timeout
- [ ] Implement `ReceiveMotion()` accessor
- [ ] Add channel buffer size configuration (64-128 commands)
- [ ] Add command routing logic (motion vs. non-motion)

### 2.2 Separate Timer Systems
- [ ] Refactor `core/scheduler.go` for per-core schedulers
- [ ] Create `CoreScheduler` struct with core ID
- [ ] Implement per-core timer queues
- [ ] Add `commScheduler` for Core 0
- [ ] Add `motionScheduler` for Core 1
- [ ] Design lock-free priority queue or min-heap
- [ ] Replace linked list timer implementation
- [ ] Benchmark timer queue performance

### 2.3 Testing
- [ ] Test channel throughput on hardware
- [ ] Test command queue under load
- [ ] Verify timer scheduler correctness
- [ ] Profile latency of channel operations

**Success Criteria**: Command queue handles >1000 commands/sec, timer dispatch <10μs

---

## Phase 3: Core Separation (Week 4-5)

### 3.1 Refactor Main Loops

#### Core 0 Implementation
- [ ] Create `targets/rp2040/core0.go`
- [ ] Implement `Core0Main()` function
- [ ] Add USB I/O handling (reuse existing goroutine)
- [ ] Add message parsing with transport.Receive()
- [ ] Implement command routing (motion vs. non-motion)
- [ ] Add event handling from Core 1
- [ ] Add output buffer flushing
- [ ] Configure Core 0 sleep interval (100μs)

#### Core 1 Implementation
- [ ] Create `targets/rp2040/core1.go`
- [ ] Implement `Core1Main()` function
- [ ] Add `runtime.LockOSThread()` for core pinning
- [ ] Implement motion command reception
- [ ] Add motion scheduling logic
- [ ] Implement timer dispatch loop
- [ ] Add endstop monitoring
- [ ] Add event sending to Core 0
- [ ] Configure Core 1 sleep interval (10μs)

#### Main Entry Point
- [ ] Update `targets/rp2040/main.go`
- [ ] Initialize command queue in main
- [ ] Launch Core 1 with goroutine
- [ ] Run Core 0 on main goroutine
- [ ] Add graceful shutdown handling

### 3.2 Command Routing
- [ ] Define motion command types (step, move, etc.)
- [ ] Implement `isMotionCommand()` classifier
- [ ] Add command validation
- [ ] Implement non-real-time command execution
- [ ] Add error handling for queue full

### 3.3 Event System
- [ ] Define event types (endstop, error, status)
- [ ] Implement endstop event generation
- [ ] Implement error event handling
- [ ] Add event logging/debugging

### 3.4 Testing
- [ ] Test basic dual-core operation
- [ ] Verify Core 1 receives motion commands
- [ ] Test endstop event propagation
- [ ] Verify non-motion commands execute on Core 0
- [ ] Test with simple motion commands

**Success Criteria**: Dual-core operation verified, commands route correctly, events propagate

---

## Phase 4: Optimization & Real-Time Tuning (Week 6)

### 4.1 Lock-Free Structures
- [ ] Create `core/lockfree_queue.go`
- [ ] Implement SPSC (Single Producer Single Consumer) queue
- [ ] Add atomic head/tail pointers
- [ ] Implement `Push()` with bounds checking
- [ ] Implement `Pop()` with empty checking
- [ ] Add benchmark comparisons vs. channels
- [ ] Consider replacing channel with SPSC queue if faster

### 4.2 Timer Optimization
- [ ] Profile timer dispatch latency
- [ ] Optimize priority queue operations
- [ ] Reduce interrupt disable duration
- [ ] Consider lock-free timer data structures
- [ ] Add timer latency measurements

### 4.3 Performance Profiling
- [ ] Add latency instrumentation to command path
- [ ] Measure Core 0 → Core 1 command latency
- [ ] Measure timer dispatch jitter
- [ ] Profile memory allocations in hot paths
- [ ] Optimize channel buffer sizes based on measurements

### 4.4 Memory Optimization
- [ ] Pre-allocate command buffers
- [ ] Reduce allocations in motion loop
- [ ] Profile TinyGo GC behavior under load
- [ ] Optimize buffer copy operations
- [ ] Add memory usage monitoring

**Success Criteria**: <10μs timer latency, >1000 cmds/sec throughput, minimal GC overhead

---

## Phase 5: Advanced Features (Week 7+)

### 5.1 Work Stealing for CPU-Bound Tasks
- [ ] Create `core/workpool.go`
- [ ] Implement `WorkPool` with task channel
- [ ] Add `Submit()` with fallback execution
- [ ] Create worker goroutines on both cores
- [ ] Offload dictionary compression to work pool
- [ ] Add work pool shutdown handling

### 5.2 Adaptive Load Balancing
- [ ] Add CPU utilization monitoring per core
- [ ] Implement dynamic task distribution
- [ ] Add load balancing heuristics
- [ ] Profile and tune balancing algorithm

### 5.3 Advanced Synchronization
- [ ] Implement wait-free algorithms for critical paths
- [ ] Add RCU (Read-Copy-Update) for read-heavy data
- [ ] Optimize cache line alignment for atomics
- [ ] Consider platform-specific optimizations

**Success Criteria**: Balanced CPU utilization, optimized for real-time performance

---

## Idiomatic Go Patterns to Use

### Share Memory by Communicating
```go
// ✅ Use channels instead of shared memory with locks
counterCh := make(chan int, 1)
counterCh <- currentValue
newValue := <-counterCh + 1
counterCh <- newValue
```

### Select for Non-Blocking Operations
```go
select {
case cmd := <-motionCh:
    handleMotion(cmd)
case event := <-eventCh:
    handleEvent(event)
default:
    // Non-blocking path
}
```

### Context for Cancellation
```go
func Core1Main(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return  // Clean shutdown
        case cmd := <-cmdQueue.motionCh:
            scheduleMotion(cmd)
        }
    }
}
```

### Worker Pools with Goroutines
```go
for i := 0; i < numCores; i++ {
    go worker(taskQueue)
}
```

---

## TinyGo-Specific Considerations

### From CLAUDE.md Notes:
1. **Channel Behavior**: May differ from standard Go
   - [ ] Add extensive channel testing on hardware
   - [ ] Compare buffered vs. unbuffered channel performance
   - [ ] Test channel behavior under high load

2. **Goroutine Scheduling**: Different characteristics
   - [ ] Profile real-time latency with goroutines
   - [ ] Test goroutine switching overhead
   - [ ] Verify core affinity works as expected

3. **Memory Management**: TinyGo GC is more aggressive
   - [ ] Pre-allocate buffers where possible
   - [ ] Avoid allocations in hot paths
   - [ ] Profile GC pauses under load

4. **Atomic Operations**: Work reliably
   - [ ] Use atomics for all shared state
   - [ ] Document atomic usage patterns
   - [ ] Test atomic operations on hardware

### Fallback Plans:
- [ ] If channels add too much latency, use lock-free queues
- [ ] If GC pauses are too long, increase heap size
- [ ] If goroutine scheduling is unpredictable, use explicit core pinning

---

## Success Metrics

- [ ] ✅ No data races (verify with tests)
- [ ] ✅ Timer dispatch latency < 10μs
- [ ] ✅ Motion command throughput > 1000 commands/sec
- [ ] ✅ CPU utilization balanced between cores
- [ ] ✅ Maintain compatibility with Klipper protocol
- [ ] ✅ Stepper pulse timing accuracy within 1μs
- [ ] ✅ No missed steps under full load

---

## Quick Start: Minimal Viable Multicore (POC)

Before full implementation, test basic multicore concept:

### POC Tasks
- [ ] Create `targets/rp2040/multicore_poc.go`
- [ ] Implement minimal dual-core test
- [ ] Move timer dispatch to separate goroutine
- [ ] Verify goroutine runs on Core 1
- [ ] Measure latency improvement
- [ ] Test on actual RP2040 hardware

```go
func MinimalMulticore() {
    cmdCh := make(chan string, 32)

    // Core 1: Timer dispatch
    go func() {
        for {
            UpdateSystemTime()
            motionScheduler.DispatchTimers()
            time.Sleep(10 * time.Microsecond)
        }
    }()

    // Core 0: Everything else
    mainLoop(cmdCh)
}
```

---

## Notes and Design Decisions

### Architecture Decisions
- **Why channels?**: Idiomatic Go, easier to reason about, prevent shared state bugs
- **Why separate schedulers?**: Avoid contention, enable core-specific optimizations
- **Why SPSC queue?**: Fallback for minimal latency if channels are too slow

### Trade-offs
- **Channels vs. Lock-free**: Channels easier to debug, lock-free faster but complex
- **Buffered vs. Unbuffered**: Buffered prevents blocking, unbuffered enforces synchronization
- **Mutex vs. Atomic**: Atomics faster for simple state, mutexes for complex operations

### Open Questions
- [ ] What is acceptable timer dispatch latency for stepper control?
- [ ] How large should command queue buffers be?
- [ ] Should endstop monitoring be interrupt-driven or polled?
- [ ] Can we eliminate interrupt disable entirely with lock-free structures?

---

## References

- [TinyGo Concurrency Docs](https://tinygo.org/docs/reference/concurrency/)
- [Go Concurrency Patterns](https://go.dev/blog/pipelines)
- [RP2040 Datasheet - Multicore](https://datasheets.raspberrypi.com/rp2040/rp2040-datasheet.pdf)
- Klipper firmware architecture (for comparison)

---

## Timeline Summary

| Phase | Duration | Deliverable |
|-------|----------|-------------|
| Phase 1 | Week 1 | Race condition fixes, atomic state |
| Phase 2 | Week 2-3 | Command queue with channels, dual schedulers |
| Phase 3 | Week 4-5 | Dual-core main loops, command routing |
| Phase 4 | Week 6 | Optimization, lock-free structures |
| Phase 5 | Week 7+ | Work pools, load balancing, advanced features |

**Total Estimated Time**: 7+ weeks for full implementation

---

**Last Updated**: 2025-11-12
**Status**: Planning Complete, Ready for Phase 1