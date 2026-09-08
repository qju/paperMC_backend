package profiler

import (
	"strings"
	"testing"
)

func TestExtractProfilerURL(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedURL  string
		expectedType string
		expectedOk   bool
	}{
		{
			name:         "Spark profile URL",
			input:        "[12:30:00 INFO]: [spark] Profile uploaded to: https://spark.lucko.me/abc123XYZ",
			expectedURL:  "https://spark.lucko.me/abc123XYZ",
			expectedType: "spark_profile",
			expectedOk:   true,
		},
		{
			name:         "Timings paste URL with punctuation",
			input:        "[12:30:05 INFO]: View Timings Report: https://timings.aikar.co/?id=def456UVW.",
			expectedURL:  "https://timings.aikar.co/?id=def456UVW",
			expectedType: "timings",
			expectedOk:   true,
		},
		{
			name:         "Timings papermc URL",
			input:        "Timings: https://timings.papermc.io/report?id=ghi789",
			expectedURL:  "https://timings.papermc.io/report?id=ghi789",
			expectedType: "timings",
			expectedOk:   true,
		},
		{
			name:         "No profiler URL present",
			input:        "[12:30:00 INFO]: Normal chat message or output without links",
			expectedURL:  "",
			expectedType: "",
			expectedOk:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, rType, ok := ExtractProfilerURL(tt.input)
			if ok != tt.expectedOk {
				t.Fatalf("Expected ok=%t, got %t", tt.expectedOk, ok)
			}
			if url != tt.expectedURL {
				t.Errorf("Expected URL %q, got %q", tt.expectedURL, url)
			}
			if rType != tt.expectedType {
				t.Errorf("Expected type %q, got %q", tt.expectedType, rType)
			}
		})
	}
}

func TestParseHealth(t *testing.T) {
	sampleSparkHealth := `
TPS from last 5s, 10s, 1m, 5m, 15m:
 20.0, 20.0, 19.95, 20.0, 20.0
Tick durations (min/med/95%ile/max ms):
 11.2 / 16.5 / 24.1 / 38.0
CPU usage:
 Process: 15.5% (last 10s), 14.2% (last 1m), 15.0% (last 15m)
 System: 25.0% (last 10s), 23.1% (last 1m), 21.8% (last 15m)
Memory usage:
 Heap: 2.5 GB / 6.0 GB (41%)
Garbage collector:
 G1 Young Generation: 15 collections, 140ms total duration (avg 9.3ms)
 G1 Old Generation: 0 collections
Disk usage:
 Root: 40.0 GB / 100.0 GB (40%)
`

	snapshot := ParseHealth(sampleSparkHealth)

	if !strings.Contains(snapshot.TPS, "20.0") {
		t.Errorf("Expected TPS to contain 20.0, got %q", snapshot.TPS)
	}
	if !strings.Contains(snapshot.MSPT, "16.5") {
		t.Errorf("Expected MSPT to contain 16.5, got %q", snapshot.MSPT)
	}
	if snapshot.CPUProcess != "15.5%" {
		t.Errorf("Expected CPUProcess 15.5%%, got %q", snapshot.CPUProcess)
	}
	if snapshot.CPUSystem != "25.0%" {
		t.Errorf("Expected CPUSystem 25.0%%, got %q", snapshot.CPUSystem)
	}
	if snapshot.MemoryUsed != "2.5 GB" || snapshot.MemoryMax != "6.0 GB" || snapshot.MemoryPercent != "41%" {
		t.Errorf("Unexpected memory heap: used=%s max=%s pct=%s", snapshot.MemoryUsed, snapshot.MemoryMax, snapshot.MemoryPercent)
	}
	if !strings.Contains(snapshot.GCSummary, "G1 Young Generation") {
		t.Errorf("Expected GCSummary to have Young Gen, got %q", snapshot.GCSummary)
	}
	if !strings.Contains(snapshot.DiskUsage, "40.0 GB") {
		t.Errorf("Expected DiskUsage to contain 40.0 GB, got %q", snapshot.DiskUsage)
	}
	if len(snapshot.Advice) == 0 {
		t.Errorf("Expected advice to be generated")
	}
}

func TestGenerateAdviceScenarios(t *testing.T) {
	// Scenario 1: Critical lag (MSPT >= 50ms)
	s1 := &HealthSnapshot{
		MSPT:          "45.0 / 58.2 / 80.0 / 120.0",
		MemoryPercent: "50%",
	}
	adv1 := GenerateAdvice(s1)
	foundLag := false
	for _, a := range adv1 {
		if strings.Contains(a, "Critical tick lag") {
			foundLag = true
		}
	}
	if !foundLag {
		t.Errorf("Expected critical tick lag advice, got %+v", adv1)
	}

	// Scenario 2: High tick load (MSPT >= 40ms)
	s2 := &HealthSnapshot{
		MSPT: "42.5ms",
	}
	adv2 := GenerateAdvice(s2)
	foundHighTick := false
	for _, a := range adv2 {
		if strings.Contains(a, "High tick load") {
			foundHighTick = true
		}
	}
	if !foundHighTick {
		t.Errorf("Expected high tick load advice, got %+v", adv2)
	}

	// Scenario 3: Memory danger (Heap >= 90%)
	s3 := &HealthSnapshot{
		MemoryPercent: "93.5%",
	}
	adv3 := GenerateAdvice(s3)
	foundHeap := false
	for _, a := range adv3 {
		if strings.Contains(a, "dangerously full") {
			foundHeap = true
		}
	}
	if !foundHeap {
		t.Errorf("Expected heap danger advice, got %+v", adv3)
	}

	// Scenario 4: Memory high (Heap >= 80%)
	s4 := &HealthSnapshot{
		MemoryPercent: "82.0%",
	}
	adv4 := GenerateAdvice(s4)
	foundHeapHigh := false
	for _, a := range adv4 {
		if strings.Contains(a, "JVM Heap is high") {
			foundHeapHigh = true
		}
	}
	if !foundHeapHigh {
		t.Errorf("Expected heap high advice, got %+v", adv4)
	}

	// Scenario 5: Full GC collections detected
	s5 := &HealthSnapshot{
		GCSummary: "G1 Old Generation: 4 collections, 450ms total",
	}
	adv5 := GenerateAdvice(s5)
	foundGC := false
	for _, a := range adv5 {
		if strings.Contains(a, "Full GC (Old Generation)") {
			foundGC = true
		}
	}
	if !foundGC {
		t.Errorf("Expected Old Gen GC advice, got %+v", adv5)
	}

	// Scenario 6: High CPU usage
	s6 := &HealthSnapshot{
		CPUProcess: "95.5%",
	}
	adv6 := GenerateAdvice(s6)
	foundCPU := false
	for _, a := range adv6 {
		if strings.Contains(a, "Process CPU usage is very high") {
			foundCPU = true
		}
	}
	if !foundCPU {
		t.Errorf("Expected high CPU advice, got %+v", adv6)
	}

	// Scenario 7: Completely empty / optimal
	s7 := &HealthSnapshot{}
	adv7 := GenerateAdvice(s7)
	if len(adv7) != 1 || !strings.Contains(adv7[0], "healthy and optimal") {
		t.Errorf("Expected healthy advice for empty snapshot, got %+v", adv7)
	}
}
