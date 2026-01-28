// Package hardware provides system hardware monitoring capabilities.
// It reads CPU frequency, temperature, and fan speeds from Linux sysfs.
package hardware

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Stats represents current hardware metrics.
type Stats struct {
	CPUFreqPct  float64 // CPU frequency percentage (current/max * 100)
	CPUFreqCur  int     // Current frequency in MHz
	CPUFreqMax  int     // Max frequency in MHz
	Temperature float64 // CPU temperature in Celsius
	FanRPMs     []int   // Fan speeds in RPM
}

// Get retrieves current hardware statistics from the system.
func Get() Stats {
	stats := Stats{}
	stats.CPUFreqCur, stats.CPUFreqMax, stats.CPUFreqPct = getCPUFrequency()
	stats.Temperature = getCPUTemperature()
	stats.FanRPMs = getFanSpeeds()
	return stats
}

// readFileInt reads an integer value from a file path.
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

// getCPUFrequency reads CPU frequency information from sysfs.
// Returns current MHz, max MHz, and percentage.
func getCPUFrequency() (cur int, max int, pct float64) {
	// Read CPU0 frequency as representative of the system
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

// getCPUTemperature reads CPU temperature from thermal zones.
// Returns temperature in Celsius, or 0 if not available.
func getCPUTemperature() float64 {
	// Try common thermal zone patterns
	patterns := []string{
		"/sys/class/thermal/thermal_zone*/temp",
		"/sys/class/hwmon/hwmon*/temp*_input",
	}

	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, path := range matches {
			// Try to read label to identify CPU sensors
			label := ""
			labelPath := filepath.Join(
				filepath.Dir(path),
				strings.TrimSuffix(filepath.Base(path), "_input")+"_label",
			)
			if data, err := os.ReadFile(labelPath); err == nil {
				label = strings.ToLower(strings.TrimSpace(string(data)))
			}

			// Filter for CPU-related sensors
			if label != "" &&
				!strings.Contains(label, "core") &&
				!strings.Contains(label, "cpu") &&
				!strings.Contains(label, "package") {
				continue
			}

			temp, err := readFileInt(path)
			if err != nil {
				continue
			}

			// Temperature is reported in millidegrees
			return float64(temp) / 1000.0
		}
	}
	return 0
}

// getFanSpeeds reads all fan speeds from hwmon sysfs entries.
// Returns a slice of RPM values for all detected fans.
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
