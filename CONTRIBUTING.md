# Contributing to goburn

Thank you for your interest in contributing to goburn! This guide will help you understand the codebase and make effective contributions.

## Code Organization Principles

### Package Separation

The codebase follows clear separation of concerns:

1. **hardware**: System interaction layer
   - Pure functions, no state
   - Linux sysfs reading only
   - No dependencies on other packages

2. **worker**: Business logic layer
   - Manages worker lifecycle
   - No UI concerns
   - No hardware reading

3. **ui**: Presentation layer
   - Uses hardware and worker packages
   - No business logic
   - Two independent implementations

4. **main**: Orchestration layer
   - Minimal logic
   - CLI flag parsing
   - Delegates to appropriate packages

### Design Patterns Used

- **Package by Feature**: Each package represents a distinct feature
- **Dependency Injection**: Pass dependencies (counter, duration) explicitly
- **Interface Segregation**: Packages export only what's needed
- **Single Responsibility**: Each file has one clear purpose

## Code Style Guide

### Naming Conventions

- **Packages**: Short, lowercase, single word (e.g., `worker`, not `workers`)
- **Exported Types**: PascalCase (e.g., `Pool`, `Stats`)
- **Unexported Functions**: camelCase (e.g., `getCPUFrequency`)
- **Constants**: PascalCase or UPPER_SNAKE_CASE for emphasis

### Documentation

Every exported symbol must have a comment:

```go
// Get retrieves current hardware statistics from the system.
func Get() Stats {
    // ...
}
```

Format:
- Start with the symbol name
- Use present tense ("retrieves", not "retrieve")
- Explain what, not how (details go in code comments)

### Error Handling

- Return errors, don't panic
- Handle errors at the appropriate level
- Log errors only when they matter
- Gracefully degrade when hardware unavailable

Example:
```go
// Good: Return zero value on error, caller decides importance
func getCPUTemperature() float64 {
    temp, err := readFileInt(path)
    if err != nil {
        return 0  // Caller checks if > 0
    }
    return float64(temp) / 1000.0
}
```

### Concurrency

- Use channels for signaling, not data sharing
- Atomic operations for counters only
- Document goroutine lifecycle
- Ensure graceful shutdown

## Testing Guidelines

### Unit Tests

Create `*_test.go` files alongside source:

```go
// hardware/stats_test.go
func TestReadFileInt(t *testing.T) {
    // Test with temp file
}
```

### Integration Tests

Test package interactions:

```go
// worker/pool_test.go
func TestPoolScaling(t *testing.T) {
    // Test worker add/remove
}
```

### Manual Testing

Always test both modes:

```bash
# Test line mode
./goburn -duration=10s

# Test graph mode
./goburn -duration=10s -graph

# Test interactive controls
# In graph mode: press +, -, verify workers change
```

## Adding Features

### Example: Adding Memory Usage Monitoring

1. **Add to hardware package**:

```go
// hardware/stats.go

type Stats struct {
    // ... existing fields ...
    MemoryUsedMB  int     // Memory used in MB
    MemoryTotalMB int     // Total memory in MB
}

func getMemoryUsage() (used, total int) {
    // Read from /proc/meminfo
    return
}

func Get() Stats {
    stats := Stats{}
    // ... existing code ...
    stats.MemoryUsedMB, stats.MemoryTotalMB = getMemoryUsage()
    return stats
}
```

2. **Add to line mode**:

```go
// ui/line.go

func formatHardwareStats(stats hardware.Stats) string {
    // ... existing code ...

    if stats.MemoryTotalMB > 0 {
        parts = append(parts, fmt.Sprintf("mem=%d/%dMB",
            stats.MemoryUsedMB, stats.MemoryTotalMB))
    }

    // ... existing code ...
}
```

3. **Add to TUI** (optional):

```go
// ui/tui.go

type Model struct {
    // ... existing fields ...
    memHistory []float64
}

// Update updateHistory()
// Update renderGraphs() to add 3rd row or replace existing graph
```

4. **Test thoroughly**:
```bash
go build
./goburn -duration=30s        # Verify line output
./goburn -duration=30s -graph # Verify TUI shows it
```

## Common Pitfalls

### Don't Mix Concerns

❌ Bad:
```go
// hardware/stats.go
func Get() Stats {
    stats := Stats{}
    // ... get stats ...
    fmt.Println("CPU:", stats.CPUFreqCur)  // NO! UI concern
    return stats
}
```

✅ Good:
```go
// hardware/stats.go
func Get() Stats {
    stats := Stats{}
    // ... get stats ...
    return stats  // Just return data
}
```

### Don't Create Circular Dependencies

❌ Bad:
```
hardware → ui → hardware  // Circular!
```

✅ Good:
```
main → ui → hardware  // One direction
     → worker
```

### Don't Hardcode UI in Business Logic

❌ Bad:
```go
// worker/pool.go
func (wp *Pool) SetWorkers(n int) {
    fmt.Printf("Setting workers to %d\n", n)  // NO!
}
```

✅ Good:
```go
// worker/pool.go
func (wp *Pool) SetWorkers(n int) {
    // Just do the work, let caller handle UI
}
```

## Pull Request Checklist

Before submitting:

- [ ] Code builds without warnings: `go build`
- [ ] Code is formatted: `go fmt ./...`
- [ ] All packages have package documentation
- [ ] Exported symbols have doc comments
- [ ] Manual testing in both modes completed
- [ ] No new dependencies added without discussion
- [ ] README.md updated if user-facing changes
- [ ] CONTRIBUTING.md updated if architecture changes

## Questions?

- Read the code! It's well-documented.
- Check existing patterns before inventing new ones
- Keep it simple - complexity is technical debt

## Performance Considerations

- Hardware stats reading: Limit to ~1 Hz, file I/O is slow
- Worker operations: Batch counter updates (current: 1M ops)
- TUI updates: 1 Hz is smooth enough, more wastes CPU
- Graph history: 60 points is sufficient, more wastes memory

## Future Ideas

Some ideas for future contributors:

- [ ] Add network I/O stress testing
- [ ] Add disk I/O stress testing
- [ ] Support macOS hardware monitoring
- [ ] Add JSON output mode for scripting
- [ ] Add configuration file support
- [ ] Add benchmark comparison mode
- [ ] Add GPU monitoring/stress testing
- [ ] Add web server mode for remote monitoring

Choose one that interests you and open an issue to discuss before implementing!
