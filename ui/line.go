// Package ui provides user interface implementations for goburn.
package ui

import (
	"fmt"
	"sync/atomic"
	"time"

	"goburn/hardware"
)

// RunLineMode displays simple line-by-line output with hardware stats.
// This is the default non-interactive mode.
func RunLineMode(counter *uint64, duration time.Duration, startTime time.Time) {
	var last uint64

	for {
		time.Sleep(time.Second)
		c := atomic.LoadUint64(counter)
		ops := (c - last) / 1_000_000
		elapsed := time.Since(startTime)

		// Get hardware stats
		hwStats := hardware.Get()
		hwInfo := formatHardwareStats(hwStats)

		fmt.Printf("[%s] ops=%dM/s%s\n",
			elapsed.Round(time.Second),
			ops,
			hwInfo)

		if elapsed >= duration {
			return
		}
		last = c
	}
}

// formatHardwareStats converts hardware stats into a readable string.
// Returns an empty string if no stats are available.
func formatHardwareStats(stats hardware.Stats) string {
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
		parts = append(parts, fmt.Sprintf("fans=%sRPM", joinStrings(fanStrs, ",")))
	}

	if len(parts) == 0 {
		return ""
	}

	return " | " + joinStrings(parts, " | ")
}

// joinStrings concatenates strings with a separator.
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
