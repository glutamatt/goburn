# goburn Architecture

## Overview

goburn is designed with clean separation of concerns, making it easy to understand, test, and extend.

## Design Philosophy

1. **Package by Feature**: Each package represents a distinct capability
2. **Single Responsibility**: Each file has one clear purpose
3. **Explicit Dependencies**: No hidden state or global variables
4. **Graceful Degradation**: Missing hardware sensors don't cause failures

## Module Structure

```
goburn/
├── main.go                  # 76 lines  - Entry point & CLI
├── hardware/
│   └── stats.go            # 121 lines - System monitoring
├── worker/
│   └── pool.go             # 112 lines - Worker management
└── ui/
    ├── line.go             # 81 lines  - Simple output
    └── tui.go              # 363 lines - Interactive TUI
```

Total: ~750 lines, well-organized and documented.

## Data Flow

```
main.go
  ├─→ worker.New()           Create worker pool
  │     └─→ worker.SetWorkers()  Spawn goroutines
  │
  ├─→ ui.RunLineMode()       OR
  │     └─→ hardware.Get()   Read stats every second
  │
  └─→ ui.RunGraphMode()
        ├─→ hardware.Get()   Read stats every second
        └─→ worker.SetWorkers()  Adjust on key press
```

## Package Details

### `main` (76 lines)

**Purpose**: Application entry point and orchestration

**Responsibilities**:
- Parse CLI flags
- Initialize operation counter
- Delegate to appropriate UI mode

**Key Functions**:
- `main()`: Entry point
- `spawnSimpleWorkers()`: Create workers for line mode

**Dependencies**: `worker`, `ui`

---

### `hardware` (121 lines)

**Purpose**: System hardware monitoring via Linux sysfs

**Responsibilities**:
- Read CPU frequency from sysfs
- Read CPU temperature from thermal zones
- Read fan speeds from hwmon
- Return structured data

**Key Types**:
```go
type Stats struct {
    CPUFreqPct  float64  // 0-100
    CPUFreqCur  int      // MHz
    CPUFreqMax  int      // MHz
    Temperature float64  // Celsius
    FanRPMs     []int    // Per-fan RPM
}
```

**Key Functions**:
- `Get()`: Main entry point, returns current stats
- `getCPUFrequency()`: Read from `/sys/devices/system/cpu/`
- `getCPUTemperature()`: Read from `/sys/class/thermal/`
- `getFanSpeeds()`: Read from `/sys/class/hwmon/`

**Dependencies**: None (standard library only)

**Platform**: Linux only (requires sysfs)

---

### `worker` (112 lines)

**Purpose**: Dynamic CPU worker pool management

**Responsibilities**:
- Spawn/stop worker goroutines
- Perform CPU-intensive operations
- Update shared operation counter
- Adjust GOMAXPROCS

**Key Types**:
```go
type Pool struct {
    counter      *uint64       // Shared op counter
    stopChannels []chan bool   // Per-worker stop signal
    activeCount  int32         // Atomic worker count
}
```

**Key Functions**:
- `New(counter, n)`: Create pool with n workers
- `SetWorkers(n)`: Adjust to exactly n workers
- `GetActiveCount()`: Current worker count
- `GetCounter()`: Access to shared counter
- `runWorker()`: Worker goroutine logic

**Algorithm**:
- Each worker performs floating-point math (`math.Pow`)
- Updates shared counter every 1M operations
- Responds to stop signal via channel

**Dependencies**: None (standard library only)

---

### `ui/line` (81 lines)

**Purpose**: Simple line-based output mode

**Responsibilities**:
- Print stats once per second
- Format hardware stats readably
- Parse-friendly output format

**Output Format**:
```
[1s] ops=142M/s | cpu=2200/4500MHz (49%) | temp=59.0C | fans=3952,4255RPM
```

**Key Functions**:
- `RunLineMode()`: Main loop
- `formatHardwareStats()`: Convert Stats to string

**Dependencies**: `hardware`

---

### `ui/tui` (363 lines)

**Purpose**: Interactive TUI with real-time graphs

**Responsibilities**:
- Display 2×2 graph layout
- Handle keyboard input
- Update graphs every second
- Dynamically resize to terminal

**Key Types**:
```go
type Model struct {
    workerPool   *worker.Pool
    currentStats hardware.Stats
    opsHistory   []float64  // Rolling 60-sec window
    cpuHistory   []float64
    tempHistory  []float64
    fanHistory   []float64
    // ... sizing and state ...
}
```

**Key Functions**:
- `RunGraphMode()`: Entry point
- `Init()`: Start tick loop
- `Update()`: Handle events (keyboard, tick, resize)
- `View()`: Render TUI
- `renderGraphs()`: Create 2×2 layout
- `renderGraph()`: Single graph panel
- `calculateGraphDimensions()`: Dynamic sizing

**Event Handling**:
- `tea.WindowSizeMsg`: Update dimensions
- `tea.KeyMsg`: Handle +, -, q
- `tickMsg`: Update stats and graphs

**Layout**:
```
┌─────────────────────────────────────┐
│ Header: Title, elapsed, workers     │
│ Stats: Ops, CPU, Temp, Fans         │
├─────────────────┬───────────────────┤
│ Ops Graph       │ CPU Freq Graph    │
├─────────────────┼───────────────────┤
│ Temp Graph      │ Fan Speed Graph   │
├─────────────────┴───────────────────┤
│ Help: Controls                      │
└─────────────────────────────────────┘
```

**Dependencies**: `worker`, `hardware`, `bubbletea`, `lipgloss`, `asciigraph`

---

## Key Design Decisions

### Why Separate hardware Package?

- **Testable**: Can mock sysfs in tests
- **Reusable**: Other tools could use this
- **Platform-specific**: Easy to add macOS/Windows versions
- **Single Responsibility**: Only reads hardware, no UI/logic

### Why Separate worker Package?

- **Testable**: Can verify worker lifecycle
- **Reusable**: Could benchmark other operations
- **Configurable**: Easy to change worker algorithm
- **Independent**: No coupling to UI or hardware

### Why Split UI into Two Files?

- **Different Concerns**: Line mode is simple, TUI is complex
- **Independent**: Can modify one without affecting other
- **Clear**: Each file has single purpose
- **Maintainable**: TUI complexity isolated

### Why Not Use Interfaces?

- **YAGNI**: No current need for multiple implementations
- **Simplicity**: Direct calls are clearer than interfaces
- **Future**: Easy to add interfaces when needed

### Why Atomic Counter Instead of Mutex?

- **Performance**: Atomic operations are faster
- **Simplicity**: Counter is the only shared state
- **Correctness**: Atomics provide sufficient guarantees

## Thread Safety

### Shared State

Only one piece of shared mutable state:
```go
var counter uint64  // Atomic operations only
```

### Synchronization Points

1. **Workers → Counter**: `atomic.AddUint64()`
2. **UI → Counter**: `atomic.LoadUint64()`
3. **Main → Workers**: Channels for stop signals

### Why This Is Safe

- Counter uses atomic operations (lock-free)
- Each worker has its own stop channel
- UI only reads, never modifies worker state
- Hardware reads are independent per call

## Error Handling Strategy

### Philosophy: Graceful Degradation

Missing hardware? → Display zeros, continue
Failed to read temp? → Skip temp graph
No fans detected? → Don't show fan stats

### Where Errors Are Handled

1. **hardware package**: Return zero values on error
2. **UI packages**: Check for zero, skip display
3. **worker package**: Panic-free, uses channels
4. **main**: Only exits on TUI initialization failure

### Why Not Return Errors?

- Hardware unavailability is expected (different systems)
- Zeros are meaningful ("not available")
- Simplifies calling code (no error checking)
- Still logged if needed (can add debug mode)

## Performance Characteristics

### CPU Usage

- **Workers**: Configurable, 100% of N cores
- **Monitoring**: ~0.1% (reads every 1 second)
- **TUI**: ~0.5% (renders at 1 Hz)

### Memory Usage

- **Base**: ~5 MB (Go runtime + binary)
- **Per Worker**: ~8 KB (goroutine stack)
- **History**: ~2 KB (60 points × 4 graphs × 8 bytes)
- **Total**: ~5-10 MB typical

### I/O Operations

- **Hardware reads**: ~10 file reads/second (throttled)
- **Terminal writes**: 1 Hz (line mode) or full redraw (TUI)

## Testing Strategy

### Unit Tests (TODO)

- `hardware/stats_test.go`: Mock sysfs files
- `worker/pool_test.go`: Verify scaling logic
- `ui/*_test.go`: Test formatters

### Integration Tests (TODO)

- End-to-end in both modes
- Worker scaling under load
- Graph rendering accuracy

### Manual Testing

Always test:
1. Line mode for 30s
2. Graph mode for 30s
3. Interactive controls (+, -, q)
4. Terminal resize in graph mode

## Extension Points

### Adding New Hardware Metrics

1. Add to `hardware.Stats` struct
2. Implement getter in `hardware/stats.go`
3. Call from `hardware.Get()`
4. Display in UI packages

### Adding New Display Mode

1. Create `ui/newmode.go`
2. Implement `RunNewMode()` function
3. Add flag in `main.go`

### Modifying Worker Behavior

Edit `worker.runWorker()` to change:
- Operation type (currently: `math.Pow`)
- Batch size (currently: 1M ops)
- Update frequency

### Platform Support

To support macOS/Windows:
- Create `hardware/stats_darwin.go` / `stats_windows.go`
- Use build tags: `//go:build darwin`
- Implement same `Get()` function

## Dependency Graph

```
main.go
  │
  ├─→ worker
  │     └─→ (stdlib)
  │
  ├─→ ui/line
  │     └─→ hardware
  │           └─→ (stdlib)
  │
  └─→ ui/tui
        ├─→ worker
        ├─→ hardware
        ├─→ bubbletea
        ├─→ lipgloss
        └─→ asciigraph
```

**Zero circular dependencies** ✓

## Future Improvements

### Short Term
- [ ] Add tests
- [ ] Add debug/verbose logging mode
- [ ] Support graceful Ctrl+C in line mode
- [ ] Add benchmark mode (compare runs)

### Medium Term
- [ ] macOS support (IOKit for hardware)
- [ ] JSON output mode
- [ ] Config file support
- [ ] Multi-core per-CPU stats

### Long Term
- [ ] Web UI for remote monitoring
- [ ] Network/disk stress testing
- [ ] GPU monitoring
- [ ] Distributed testing mode

## Conclusion

The architecture prioritizes:
1. **Clarity**: Each package has obvious purpose
2. **Maintainability**: Changes are localized
3. **Testability**: Packages are independent
4. **Extensibility**: New features fit naturally

A developer should be able to understand any part of the system in under 5 minutes.
