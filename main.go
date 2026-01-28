package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

func main() {

	duration := flag.Duration("duration", 50*time.Second, "Test duration")
	flag.Parse()

	var counter uint64
	gomaxprocs := runtime.GOMAXPROCS(-1)
	fmt.Printf("runtime.GOMAXPROCS=%d so let's spawn %d goroutines\n", gomaxprocs, gomaxprocs)
	for w := 0; w < gomaxprocs; w++ {
		go func(w int) {
			v := rand.Float64()
			var i uint64

			for {
				i++
				v *= math.Pow(v, v)
				if i%1_000_000 == 0 {
					atomic.AddUint64(&counter, i)
					i = 0
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
		since := time.Since(start)

		// Get hardware stats
		hwStats := getHardwareStats()
		hwInfo := formatHardwareStats(hwStats)

		fmt.Printf("[%s] ops=%dM/s%s\n",
			since.Round(time.Second),
			n/1_000_000,
			hwInfo)

		if since >= *duration {
			return
		}
		last = c
	}
}

type HardwareStats struct {
	CPUFreqPct  float64 // CPU frequency percentage (current/max * 100)
	CPUFreqCur  int     // Current frequency in MHz
	CPUFreqMax  int     // Max frequency in MHz
	Temperature float64 // CPU temperature in Celsius
	FanRPMs     []int   // Fan speeds in RPM
}

func readFileInt(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	val, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}
	return val, nil
}

func getCPUFrequency() (cur int, max int, pct float64) {
	// Try to read CPU0 frequency as representative
	curKHz, err := readFileInt("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq")
	if err != nil {
		return 0, 0, 0
	}
	maxKHz, err := readFileInt("/sys/devices/system/cpu/cpu0/cpufreq/scaling_max_freq")
	if err != nil {
		return 0, 0, 0
	}

	cur = curKHz / 1000 // Convert to MHz
	max = maxKHz / 1000
	if max > 0 {
		pct = float64(curKHz) / float64(maxKHz) * 100
	}
	return
}

func getCPUTemperature() float64 {
	// Try common thermal zones
	patterns := []string{
		"/sys/class/thermal/thermal_zone*/temp",
		"/sys/class/hwmon/hwmon*/temp*_input",
	}

	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, path := range matches {
			// Skip non-CPU sensors if possible
			label := ""
			labelPath := filepath.Join(filepath.Dir(path), strings.TrimSuffix(filepath.Base(path), "_input")+"_label")
			if data, err := os.ReadFile(labelPath); err == nil {
				label = strings.ToLower(strings.TrimSpace(string(data)))
			}

			// Try to find CPU-related temperature
			if label != "" && !strings.Contains(label, "core") && !strings.Contains(label, "cpu") && !strings.Contains(label, "package") {
				continue
			}

			temp, err := readFileInt(path)
			if err != nil {
				continue
			}

			// Temperature is in millidegrees
			return float64(temp) / 1000.0
		}
	}
	return 0
}

func getFanSpeeds() []int {
	var fans []int
	matches, _ := filepath.Glob("/sys/class/hwmon/hwmon*/fan*_input")

	for _, path := range matches {
		rpm, err := readFileInt(path)
		if err != nil || rpm == 0 {
			continue
		}
		fans = append(fans, rpm)
	}

	return fans
}

func getHardwareStats() HardwareStats {
	stats := HardwareStats{}
	stats.CPUFreqCur, stats.CPUFreqMax, stats.CPUFreqPct = getCPUFrequency()
	stats.Temperature = getCPUTemperature()
	stats.FanRPMs = getFanSpeeds()
	return stats
}

func formatHardwareStats(stats HardwareStats) string {
	parts := []string{}

	// CPU frequency
	if stats.CPUFreqMax > 0 {
		parts = append(parts, fmt.Sprintf("cpu=%d/%dMHz (%.0f%%)",
			stats.CPUFreqCur, stats.CPUFreqMax, stats.CPUFreqPct))
	}

	// Temperature
	if stats.Temperature > 0 {
		parts = append(parts, fmt.Sprintf("temp=%.1fC", stats.Temperature))
	}

	// Fan speeds
	if len(stats.FanRPMs) > 0 {
		fanStrs := make([]string, len(stats.FanRPMs))
		for i, rpm := range stats.FanRPMs {
			fanStrs[i] = fmt.Sprintf("%d", rpm)
		}
		parts = append(parts, fmt.Sprintf("fans=%sRPM", strings.Join(fanStrs, ",")))
	}

	if len(parts) == 0 {
		return ""
	}

	return " | " + strings.Join(parts, " | ")
}
