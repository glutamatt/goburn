// Package worker manages a dynamic pool of CPU-intensive workers.
// Workers can be added or removed at runtime to control CPU load.
package worker

import (
	"math"
	"math/rand"
	"runtime"
	"sync/atomic"
)

// Pool manages a collection of CPU-intensive worker goroutines.
// Workers can be dynamically added or removed to adjust CPU load.
type Pool struct {
	counter      *uint64      // Shared operation counter
	stopChannels []chan bool  // Stop signals for each worker
	activeCount  int32        // Current number of active workers
}

// New creates a new worker pool with the specified number of initial workers.
// The counter parameter is a shared atomic counter that workers increment.
func New(counter *uint64, initialWorkers int) *Pool {
	wp := &Pool{
		counter:      counter,
		stopChannels: make([]chan bool, 0),
		activeCount:  0,
	}
	wp.SetWorkers(initialWorkers)
	return wp
}

// SetWorkers adjusts the pool to have exactly the target number of workers.
// If target > current, new workers are spawned.
// If target < current, excess workers are stopped gracefully.
func (wp *Pool) SetWorkers(target int) {
	current := int(atomic.LoadInt32(&wp.activeCount))

	if target > current {
		// Spawn additional workers
		wp.spawnWorkers(target - current)
	} else if target < current {
		// Stop excess workers
		wp.stopWorkers(current - target)
	}

	// Update runtime GOMAXPROCS to match worker count
	runtime.GOMAXPROCS(target)
}

// GetActiveCount returns the current number of active workers.
func (wp *Pool) GetActiveCount() int {
	return int(atomic.LoadInt32(&wp.activeCount))
}

// GetCounter returns a pointer to the shared operation counter.
func (wp *Pool) GetCounter() *uint64 {
	return wp.counter
}

// spawnWorkers creates n new worker goroutines.
func (wp *Pool) spawnWorkers(n int) {
	for i := 0; i < n; i++ {
		stopCh := make(chan bool, 1)
		wp.stopChannels = append(wp.stopChannels, stopCh)
		atomic.AddInt32(&wp.activeCount, 1)

		go wp.runWorker(stopCh)
	}
}

// stopWorkers signals n workers to stop and removes their channels.
func (wp *Pool) stopWorkers(n int) {
	current := int(atomic.LoadInt32(&wp.activeCount))

	for i := 0; i < n && i < len(wp.stopChannels); i++ {
		idx := current - 1 - i
		if idx >= 0 && idx < len(wp.stopChannels) {
			wp.stopChannels[idx] <- true
			atomic.AddInt32(&wp.activeCount, -1)
		}
	}

	// Clean up stopped channels
	newCount := int(atomic.LoadInt32(&wp.activeCount))
	if newCount >= 0 && newCount < len(wp.stopChannels) {
		wp.stopChannels = wp.stopChannels[:newCount]
	}
}

// runWorker executes CPU-intensive operations until signaled to stop.
// It performs floating-point math operations to generate CPU load.
func (wp *Pool) runWorker(stopCh chan bool) {
	v := rand.Float64()
	var i uint64

	for {
		select {
		case <-stopCh:
			return
		default:
			// CPU-intensive floating-point operation
			i++
			v *= math.Pow(v, v)

			// Periodically update the shared counter
			if i%1_000_000 == 0 {
				atomic.AddUint64(wp.counter, i)
				i = 0
			}
		}
	}
}
