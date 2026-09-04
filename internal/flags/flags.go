package flags

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	PresetAikar   = "aikar"
	PresetMinimal = "minimal"
	PresetNone    = "none"
	PresetCustom  = "custom"

	DefaultRAM    = "8G"
	DefaultPreset = PresetAikar
)

// PresetInfo describes an available JVM flag preset.
type PresetInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	DocURL      string   `json:"doc_url"`
	SampleFlags []string `json:"sample_flags"`
}

var ramPattern = regexp.MustCompile(`^([0-9]+)\s*([gGmM]?)$`)

// ParseRAMToMB converts human-readable RAM strings (e.g. "8G", "4096M", "8") into megabytes.
func ParseRAMToMB(ramStr string) (int, error) {
	clean := strings.TrimSpace(ramStr)
	if clean == "" {
		return 8192, nil
	}

	matches := ramPattern.FindStringSubmatch(clean)
	if len(matches) < 2 {
		return 0, fmt.Errorf("invalid RAM format '%s'. Expected format like '8G' or '4096M'", ramStr)
	}

	val, err := strconv.Atoi(matches[1])
	if err != nil || val <= 0 {
		return 0, fmt.Errorf("invalid RAM value '%s'", ramStr)
	}

	unit := "G"
	if len(matches) >= 3 && matches[2] != "" {
		unit = strings.ToUpper(matches[2])
	}

	var mb int
	switch unit {
	case "G":
		mb = val * 1024
	case "M":
		mb = val
	default:
		mb = val * 1024
	}

	if mb < 1024 {
		return 0, fmt.Errorf("minimum allocated RAM is 1G (1024M), got %dM", mb)
	}
	if mb > 1048576 { // 1024 GB
		return 0, fmt.Errorf("maximum allocated RAM is 1024G, got %dM", mb)
	}

	return mb, nil
}

// ValidateRAM checks if a RAM string conforms to acceptable ranges (>=1G, <=1024G).
func ValidateRAM(ramStr string) error {
	_, err := ParseRAMToMB(ramStr)
	return err
}

// GetAikarFlags returns the optimized Aikar G1GC flag set dynamically tuned
// for either <12GB or >=12GB heap allocations.
func GetAikarFlags(ramMB int) []string {
	flags := []string{
		"-XX:+UseG1GC",
		"-XX:+ParallelRefProcEnabled",
		"-XX:MaxGCPauseMillis=200",
		"-XX:+UnlockExperimentalVMOptions",
		"-XX:+DisableExplicitGC",
		"-XX:+AlwaysPreTouch",
	}

	if ramMB >= 12*1024 { // >= 12GB heap
		flags = append(flags,
			"-XX:G1NewSizePercent=40",
			"-XX:G1MaxNewSizePercent=50",
			"-XX:G1ReservePercent=15",
			"-XX:InitiatingHeapOccupancyPercent=20",
		)
	} else { // < 12GB heap
		flags = append(flags,
			"-XX:G1NewSizePercent=30",
			"-XX:G1MaxNewSizePercent=40",
			"-XX:G1ReservePercent=20",
			"-XX:InitiatingHeapOccupancyPercent=15",
		)
	}

	flags = append(flags,
		"-XX:G1HeapWastePercent=5",
		"-XX:G1MixedGCCountTarget=4",
		"-XX:G1MixedGCLiveThresholdPercent=90",
		"-XX:G1RSetUpdatingPauseTimePercent=5",
		"-XX:SurvivorRatio=32",
		"-XX:+PerfDisableSharedMem",
		"-XX:MaxTenuringThreshold=1",
		"-Dusing.aikars.flags=https://mcflags.emc.gs",
		"-Daikars.new.flags=true",
	)

	return flags
}

// ParseFlagsString tokenizes a whitespace/newline separated custom flags string into a clean slice.
func ParseFlagsString(raw string) []string {
	fields := strings.Fields(raw)
	result := make([]string, 0, len(fields))
	for _, f := range fields {
		trimmed := strings.TrimSpace(f)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// GetEffectiveFlags resolves the exact JVM flags slice for a given RAM amount, preset, and custom string.
func GetEffectiveFlags(ramStr, preset, customFlags string) []string {
	ramMB, _ := ParseRAMToMB(ramStr)
	if ramMB <= 0 {
		ramMB = 8192
	}

	switch strings.ToLower(strings.TrimSpace(preset)) {
	case PresetAikar, "":
		return GetAikarFlags(ramMB)
	case PresetMinimal:
		return []string{"-XX:+UseG1GC"}
	case PresetNone:
		return []string{}
	case PresetCustom:
		return ParseFlagsString(customFlags)
	default:
		return GetAikarFlags(ramMB)
	}
}

// BuildJavaArgs constructs the full arguments slice for exec.CommandContext.
func BuildJavaArgs(ramStr, preset, customFlags, jarFile string) []string {
	cleanRAM := strings.TrimSpace(ramStr)
	if cleanRAM == "" {
		cleanRAM = DefaultRAM
	}

	args := []string{
		"-Xms" + cleanRAM,
		"-Xmx" + cleanRAM,
	}

	flags := GetEffectiveFlags(cleanRAM, preset, customFlags)
	args = append(args, flags...)

	cleanJar := strings.TrimSpace(jarFile)
	if cleanJar == "" {
		cleanJar = "server.jar"
	}
	args = append(args, "-jar", cleanJar, "nogui")

	return args
}

// GetAvailablePresets returns metadata for all available presets for UI display.
func GetAvailablePresets(ramStr string) []PresetInfo {
	ramMB, _ := ParseRAMToMB(ramStr)
	if ramMB <= 0 {
		ramMB = 8192
	}

	aikarDescription := "Industry standard PaperMC garbage collection flags. Automatically tuned for <12GB allocations to eliminate GC stutter."
	if ramMB >= 12*1024 {
		aikarDescription = "Industry standard PaperMC garbage collection flags. Automatically tuned for >=12GB allocations (larger young gen and lower GC trigger thresholds)."
	}

	return []PresetInfo{
		{
			ID:          PresetAikar,
			Name:        "Aikar's Flags (Recommended)",
			Description: aikarDescription,
			DocURL:      "https://docs.papermc.io/paper/aikars-flags",
			SampleFlags: GetAikarFlags(ramMB),
		},
		{
			ID:          PresetMinimal,
			Name:        "Minimal G1GC",
			Description: "Basic standard G1 Garbage Collector without advanced parameter tuning.",
			DocURL:      "https://docs.oracle.com/javase/8/docs/technotes/guides/vm/g1gc.html",
			SampleFlags: []string{"-XX:+UseG1GC"},
		},
		{
			ID:          PresetNone,
			Name:        "Vanilla Default (None)",
			Description: "Zero extra JVM flags. Only -Xms and -Xmx are passed to Java.",
			DocURL:      "",
			SampleFlags: []string{},
		},
		{
			ID:          PresetCustom,
			Name:        "Custom Flags",
			Description: "Full manual control. Enter your own specialized JVM flags or profiling agents.",
			DocURL:      "",
			SampleFlags: []string{},
		},
	}
}
