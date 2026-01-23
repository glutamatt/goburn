package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"sync/atomic"
	"time"
)

func main() {
	// Command-line flags
	intensity := flag.Int("intensity", 100, "CPU load intensity (0-100%)")
	duration := flag.Int("duration", 50, "Test duration in seconds")
	flag.Parse()

	// Validate intensity
	if *intensity < 0 || *intensity > 100 {
		fmt.Println("Error: intensity must be between 0 and 100")
		return
	}

	var counter uint64
	cpus := runtime.NumCPU()
	fmt.Printf("%d cpus so let's spawn %d goroutines at %d%% intensity\n", cpus, cpus, *intensity)

	// Calculate work/sleep cycle (in microseconds)
	// For 100% intensity: always work, never sleep
	// For 50% intensity: work 5ms, sleep 5ms
	// For 30% intensity: work 3ms, sleep 7ms
	workMicros := int64(*intensity * 100)   // work time in microseconds per 10ms cycle
	sleepMicros := int64((100 - *intensity) * 100) // sleep time in microseconds per 10ms cycle

	for w := 0; w < cpus; w++ {
		go func(w int) {
			v := rand.Float64()
			var i uint64
			cycleStart := time.Now()

			for {
				i++
				v *= math.Pow(v, v)
				if i%1_000_000 == 0 {
					atomic.AddUint64(&counter, i)
					i = 0
				}

				// Intensity control: periodically sleep based on intensity setting
				if *intensity < 100 && time.Since(cycleStart).Microseconds() >= workMicros {
					if sleepMicros > 0 {
						time.Sleep(time.Duration(sleepMicros) * time.Microsecond)
					}
					cycleStart = time.Now()
				}
			}
		}(w)
	}

	var last uint64
	start := time.Now()

	for {
		time.Sleep(time.Second)
		c := atomic.AddUint64(&counter, 0)
		n := c - last
		sec := time.Since(start).Seconds()
		fmt.Printf("[ %2.1fs ] %d M/sec\n", sec, n/1_000_000)
		if sec > float64(*duration) {
			return
		}
		last = c
	}
}
