package profiler

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	reSparkURL   = regexp.MustCompile(`https?://spark\.lucko\.me/[a-zA-Z0-9]+`)
	reTimingsURL = regexp.MustCompile(`https?://timings\.(?:aikar\.co|papermc\.io)/\S+`)

	reTPS       = regexp.MustCompile(`(?i)TPS(?: from last [^:]+)?:?\s*([\d.,\s]+)`)
	reTickMSPT  = regexp.MustCompile(`(?i)(?:Tick durations|Tick duration|MSPT|Average tick time)[^:\d]*:?\s*([\d./\s]+(?:\(target:[^\)]+\))?)`)
	reCPUProc   = regexp.MustCompile(`(?i)Process:\s*([\d.]+\s*%)`)
	reCPUSys    = regexp.MustCompile(`(?i)System:\s*([\d.]+\s*%)`)
	reCPUDual   = regexp.MustCompile(`(?i)CPU[^:\d]*:\s*([\d.]+\s*%)\s*/\s*([\d.]+\s*%)`)
	reHeap      = regexp.MustCompile(`(?i)Heap:\s*([\d.]+\s*(?:GB|MB|KB|B))\s*/\s*([\d.]+\s*(?:GB|MB|KB|B))(?:\s*\(([\d.]+)%\))?`)
	reGC        = regexp.MustCompile(`(?i)(G1 [^\r\n]+|ZGC [^\r\n]+|Parallel [^\r\n]+|Young Generation[^\r\n]+)`)
	reDisk      = regexp.MustCompile(`(?i)(?:Root|Disk|Storage)[^:\d]*:\s*([\d.]+\s*(?:GB|MB|KB|B)\s*/\s*[\d.]+\s*(?:GB|MB|KB|B)(?:\s*\([\d.]+%\))?)`)
)

// HealthSnapshot represents parsed server health telemetry from Spark or Paper logs.
type HealthSnapshot struct {
	TPS           string   `json:"tps"`
	MSPT          string   `json:"mspt"`
	CPUProcess    string   `json:"cpu_process"`
	CPUSystem     string   `json:"cpu_system"`
	MemoryUsed    string   `json:"memory_used"`
	MemoryMax     string   `json:"memory_max"`
	MemoryPercent string   `json:"memory_percent"`
	GCSummary     string   `json:"gc_summary"`
	DiskUsage     string   `json:"disk_usage"`
	Advice        []string `json:"advice"`
	Raw           string   `json:"raw"`
}

// ExtractProfilerURL scans a console line or text for spark or timings generated URLs.
func ExtractProfilerURL(text string) (url string, reportType string, found bool) {
	if m := reSparkURL.FindString(text); m != "" {
		return m, "spark_profile", true
	}
	if m := reTimingsURL.FindString(text); m != "" {
		// Clean trailing punctuation
		clean := strings.TrimRight(m, ".,;!?)")
		return clean, "timings", true
	}
	return "", "", false
}

// ParseHealth parses raw console output from /spark health or Paper timings into a structured snapshot.
func ParseHealth(raw string) *HealthSnapshot {
	s := &HealthSnapshot{
		Raw: raw,
	}

	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		trim := strings.TrimSpace(line)

		// 1. TPS
		if s.TPS == "" && strings.HasPrefix(strings.ToLower(trim), "tps") {
			parts := strings.SplitN(trim, ":", 2)
			val := ""
			if len(parts) > 1 {
				val = strings.TrimSpace(parts[1])
			}
			if val == "" && i+1 < len(lines) {
				val = strings.TrimSpace(lines[i+1])
			}
			if val != "" {
				s.TPS = val
			}
		}

		// 2. MSPT
		lower := strings.ToLower(trim)
		if s.MSPT == "" && (strings.HasPrefix(lower, "tick duration") || strings.HasPrefix(lower, "mspt") || strings.HasPrefix(lower, "average tick time")) {
			parts := strings.SplitN(trim, ":", 2)
			val := ""
			if len(parts) > 1 {
				val = strings.TrimSpace(parts[1])
			}
			if val == "" && i+1 < len(lines) {
				val = strings.TrimSpace(lines[i+1])
			}
			if val != "" {
				s.MSPT = val
			}
		}

		// 3. CPU
		if s.CPUProcess == "" {
			if m := reCPUProc.FindStringSubmatch(trim); len(m) > 1 {
				s.CPUProcess = m[1]
			}
		}
		if s.CPUSystem == "" {
			if m := reCPUSys.FindStringSubmatch(trim); len(m) > 1 {
				s.CPUSystem = m[1]
			}
		}
		if s.CPUProcess == "" && s.CPUSystem == "" {
			if m := reCPUDual.FindStringSubmatch(trim); len(m) > 2 {
				s.CPUProcess = m[1]
				s.CPUSystem = m[2]
			}
		}

		// 4. Memory Heap
		if s.MemoryUsed == "" {
			if m := reHeap.FindStringSubmatch(trim); len(m) > 2 {
				s.MemoryUsed = m[1]
				s.MemoryMax = m[2]
				if len(m) > 3 && m[3] != "" {
					s.MemoryPercent = m[3] + "%"
				}
			}
		}

		// 5. GC
		if m := reGC.FindStringSubmatch(trim); len(m) > 1 {
			if s.GCSummary == "" {
				s.GCSummary = strings.TrimSpace(m[1])
			} else {
				s.GCSummary += "; " + strings.TrimSpace(m[1])
			}
		}

		// 6. Disk
		if s.DiskUsage == "" {
			if m := reDisk.FindStringSubmatch(trim); len(m) > 1 {
				s.DiskUsage = strings.TrimSpace(m[1])
			}
		}
	}

	s.Advice = GenerateAdvice(s)
	return s
}

// GenerateAdvice analyzes snapshot metrics and produces actionable administrator tuning tips.
func GenerateAdvice(s *HealthSnapshot) []string {
	var advice []string

	// Analyze MSPT
	if s.MSPT != "" {
		parts := strings.Split(s.MSPT, "/")
		targetPart := s.MSPT
		if len(parts) >= 2 {
			targetPart = strings.TrimSpace(parts[1]) // median
		}
		targetPart = strings.TrimRight(targetPart, "ms")
		targetPart = strings.TrimSpace(targetPart)
		if val, err := strconv.ParseFloat(targetPart, 64); err == nil {
			if val >= 50.0 {
				advice = append(advice, fmt.Sprintf("Critical tick lag: MSPT is %.1f ms (target < 50 ms). Server is skipping ticks. Run a full Spark profile to locate laggy entities or plugins.", val))
			} else if val >= 40.0 {
				advice = append(advice, fmt.Sprintf("High tick load: MSPT is %.1f ms approaching 50 ms budget. Consider optimizing redstone and entity tracking range in paper-world-defaults.yml.", val))
			} else {
				advice = append(advice, fmt.Sprintf("Healthy tick loop: MSPT is %.1f ms with ample headroom under the 50 ms per-tick limit.", val))
			}
		}
	}

	// Analyze Memory
	if s.MemoryPercent != "" {
		pctStr := strings.TrimRight(s.MemoryPercent, "%")
		if pct, err := strconv.ParseFloat(pctStr, 64); err == nil {
			if pct >= 90.0 {
				advice = append(advice, fmt.Sprintf("JVM Heap is dangerously full (%.1f%%). High risk of OutOfMemory crash. Increase RAM in Server Flags immediately.", pct))
			} else if pct >= 80.0 {
				advice = append(advice, fmt.Sprintf("JVM Heap is high (%.1f%%). If frequent GC pauses occur, consider allocating an additional 2GB RAM.", pct))
			}
		}
	}

	// Analyze GC
	if strings.Contains(s.GCSummary, "Old Generation") && !strings.Contains(s.GCSummary, "0 collections") {
		advice = append(advice, "Full GC (Old Generation) collections detected. Ensure you are using the 'Aikar Flags' preset in Server Flags for optimized garbage collection.")
	}

	// Analyze CPU
	if s.CPUProcess != "" {
		procStr := strings.TrimRight(s.CPUProcess, "% ")
		if cpu, err := strconv.ParseFloat(procStr, 64); err == nil && cpu > 90.0 {
			advice = append(advice, fmt.Sprintf("Process CPU usage is very high (%.1f%%). Check for infinite loops in custom scripts or heavy chunk generation.", cpu))
		}
	}

	if len(advice) == 0 {
		advice = append(advice, "Server vitals look healthy and optimal.")
	}

	return advice
}
