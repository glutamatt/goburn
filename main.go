// goburn is a CPU burn testing tool with hardware monitoring.
//
// It spawns worker goroutines that perform CPU-intensive operations,
// while monitoring CPU frequency, temperature, and fan speeds.
//
// Usage:
//
//	goburn [flags]
//
// Flags:
//
//	-duration duration
//	    Test duration (default 50s)
//	-graph
//	    Enable dynamic TUI graph mode (default false)
//
// In graph mode, you can:
//   - Press '+' to increase workers
//   - Press '-' to decrease workers
//   - Press 'q' or Ctrl+C to quit
//
// Examples:
//
//	# Run for 1 minute with line output
//	goburn -duration=1m
//
//	# Run with interactive TUI graphs
//	goburn -duration=2m -graph
package main

import (
	"flag"
	"fmt"
	"runtime"
	"time"

	"goburn/ui"
	"goburn/worker"
)

func main() {
	// Parse command-line flags
	duration := flag.Duration("duration", 50*time.Second, "Test duration")
	graphMode := flag.Bool("graph", false, "Enable dynamic TUI graph mode")
	flag.Parse()

	// Initialize operation counter
	var counter uint64

	// Determine initial worker count based on available CPUs
	initialWorkers := runtime.GOMAXPROCS(-1)
	fmt.Printf("runtime.GOMAXPROCS=%d so let's spawn %d goroutines\n",
		initialWorkers, initialWorkers)

	// Wait briefly for output to be visible before TUI takes over
	time.Sleep(100 * time.Millisecond)

	start := time.Now()

	if *graphMode {
		// Interactive TUI mode with graphs
		ui.RunGraphMode(&counter, *duration, start, initialWorkers)
	} else {
		// Simple line mode
		spawnSimpleWorkers(&counter, initialWorkers)
		ui.RunLineMode(&counter, *duration, start)
	}
}

// spawnSimpleWorkers creates worker goroutines for line mode.
// In line mode, workers run for the entire duration without dynamic control.
func spawnSimpleWorkers(counter *uint64, count int) {
	wp := worker.New(counter, count)
	// Workers will run until program exits
	_ = wp
}
