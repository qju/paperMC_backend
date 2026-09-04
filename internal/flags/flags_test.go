package flags

import (
	"strings"
	"testing"
)

func TestParseRAMToMB(t *testing.T) {
	tests := []struct {
		input       string
		expectedMB  int
		shouldError bool
	}{
		{"8G", 8192, false},
		{"8g", 8192, false},
		{"4G", 4096, false},
		{"12G", 12288, false},
		{"2048M", 2048, false},
		{"2048m", 2048, false},
		{"", 8192, false}, // default fallback
		{"16", 16384, false},
		{"512M", 0, true}, // < 1024M
		{"0G", 0, true},
		{"-4G", 0, true},
		{"invalid", 0, true},
		{"2000000G", 0, true}, // > 1024G
	}

	for _, tc := range tests {
		mb, err := ParseRAMToMB(tc.input)
		if tc.shouldError {
			if err == nil {
				t.Errorf("Expected error for ParseRAMToMB(%q), got mb=%d", tc.input, mb)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for ParseRAMToMB(%q): %v", tc.input, err)
			}
			if mb != tc.expectedMB {
				t.Errorf("ParseRAMToMB(%q) = %d; want %d", tc.input, mb, tc.expectedMB)
			}
		}
	}
}

func TestValidateRAM(t *testing.T) {
	if err := ValidateRAM("8G"); err != nil {
		t.Errorf("Expected 8G to be valid: %v", err)
	}
	if err := ValidateRAM("invalid"); err == nil {
		t.Errorf("Expected error for 'invalid'")
	}
}

func TestAikarFlagsRAMTuning(t *testing.T) {
	// 1. Under 12GB (e.g. 8GB = 8192MB)
	flags8G := GetAikarFlags(8192)
	joined8G := strings.Join(flags8G, " ")

	if !strings.Contains(joined8G, "-XX:G1NewSizePercent=30") {
		t.Errorf("Expected 8G Aikar flags to contain -XX:G1NewSizePercent=30")
	}
	if !strings.Contains(joined8G, "-XX:G1ReservePercent=20") {
		t.Errorf("Expected 8G Aikar flags to contain -XX:G1ReservePercent=20")
	}
	if !strings.Contains(joined8G, "-XX:InitiatingHeapOccupancyPercent=15") {
		t.Errorf("Expected 8G Aikar flags to contain -XX:InitiatingHeapOccupancyPercent=15")
	}

	// 2. 12GB or higher (e.g. 16GB = 16384MB)
	flags16G := GetAikarFlags(16384)
	joined16G := strings.Join(flags16G, " ")

	if !strings.Contains(joined16G, "-XX:G1NewSizePercent=40") {
		t.Errorf("Expected 16G Aikar flags to contain -XX:G1NewSizePercent=40")
	}
	if !strings.Contains(joined16G, "-XX:G1ReservePercent=15") {
		t.Errorf("Expected 16G Aikar flags to contain -XX:G1ReservePercent=15")
	}
	if !strings.Contains(joined16G, "-XX:InitiatingHeapOccupancyPercent=20") {
		t.Errorf("Expected 16G Aikar flags to contain -XX:InitiatingHeapOccupancyPercent=20")
	}
}

func TestGetEffectiveFlagsPresets(t *testing.T) {
	// Aikar
	aikar := GetEffectiveFlags("8G", PresetAikar, "")
	if len(aikar) == 0 || !strings.Contains(aikar[0], "UseG1GC") {
		t.Errorf("Expected Aikar flags slice, got: %v", aikar)
	}

	// Minimal
	minimal := GetEffectiveFlags("8G", PresetMinimal, "")
	if len(minimal) != 1 || minimal[0] != "-XX:+UseG1GC" {
		t.Errorf("Expected minimal flags [-XX:+UseG1GC], got: %v", minimal)
	}

	// None
	none := GetEffectiveFlags("8G", PresetNone, "")
	if len(none) != 0 {
		t.Errorf("Expected empty slice for PresetNone, got: %v", none)
	}

	// Custom
	custom := GetEffectiveFlags("8G", PresetCustom, "-XX:+UnlockDiagnosticVMOptions\n-XX:+DebugNonSafepoints")
	if len(custom) != 2 || custom[0] != "-XX:+UnlockDiagnosticVMOptions" || custom[1] != "-XX:+DebugNonSafepoints" {
		t.Errorf("Expected custom flags, got: %v", custom)
	}

	// Unknown preset falls back to Aikar
	unknown := GetEffectiveFlags("8G", "unknown_preset", "")
	if len(unknown) != len(aikar) {
		t.Errorf("Expected fallback to Aikar for unknown preset, got %d flags", len(unknown))
	}
}

func TestBuildJavaArgs(t *testing.T) {
	args := BuildJavaArgs("10G", PresetAikar, "", "paper-1.21.jar")

	if len(args) < 6 {
		t.Fatalf("Expected at least 6 args, got %d", len(args))
	}

	if args[0] != "-Xms10G" || args[1] != "-Xmx10G" {
		t.Errorf("Expected -Xms10G and -Xmx10G at start, got %v", args[:2])
	}

	lastThree := args[len(args)-3:]
	if lastThree[0] != "-jar" || lastThree[1] != "paper-1.21.jar" || lastThree[2] != "nogui" {
		t.Errorf("Expected trailing [-jar paper-1.21.jar nogui], got %v", lastThree)
	}

	// Default fallbacks
	defaultArgs := BuildJavaArgs("", "", "", "")
	if defaultArgs[0] != "-Xms8G" || defaultArgs[1] != "-Xmx8G" {
		t.Errorf("Expected default RAM 8G, got %v", defaultArgs[:2])
	}
	if defaultArgs[len(defaultArgs)-2] != "server.jar" {
		t.Errorf("Expected default jar server.jar, got %s", defaultArgs[len(defaultArgs)-2])
	}
}

func TestGetAvailablePresets(t *testing.T) {
	presets := GetAvailablePresets("8G")
	if len(presets) != 4 {
		t.Fatalf("Expected 4 presets, got %d", len(presets))
	}

	foundAikar := false
	for _, p := range presets {
		if p.ID == PresetAikar {
			foundAikar = true
			if !strings.Contains(p.Description, "<12GB") {
				t.Errorf("Expected 8G Aikar preset description to mention <12GB, got %s", p.Description)
			}
		}
	}
	if !foundAikar {
		t.Errorf("Preset 'aikar' not found in available presets")
	}

	// Verify >=12GB description changes
	presets16G := GetAvailablePresets("16G")
	for _, p := range presets16G {
		if p.ID == PresetAikar {
			if !strings.Contains(p.Description, ">=12GB") {
				t.Errorf("Expected 16G Aikar preset description to mention >=12GB, got %s", p.Description)
			}
		}
	}
}
