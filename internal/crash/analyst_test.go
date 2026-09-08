package crash

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAnalyzeCategories(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		expectedCategory string
		expectedCulprit  string
	}{
		{
			name:             "Empty log",
			input:            "   \n\t  ",
			expectedCategory: CategoryUnknown,
			expectedCulprit:  "No output recorded",
		},
		{
			name:             "EULA unaccepted",
			input:            "[Server thread/INFO]: You need to agree to the EULA in order to run the server. Go to eula.txt for more info.",
			expectedCategory: CategoryEulaUnaccepted,
			expectedCulprit:  "eula.txt",
		},
		{
			name:             "Java version mismatch",
			input:            "java.lang.UnsupportedClassVersionError: net/minecraft/server/Main has been compiled by a more recent version of the Java Runtime (class file version 65.0), this version of the Java Runtime only recognizes class file versions up to 61.0",
			expectedCategory: CategoryJavaVersionMismatch,
			expectedCulprit:  "Requires Java 21 (class file 65.0)",
		},
		{
			name:             "Java version mismatch with other version",
			input:            "java.lang.UnsupportedClassVersionError: SomeClass has been compiled by class file version 55.0",
			expectedCategory: CategoryJavaVersionMismatch,
			expectedCulprit:  "Requires Java 11 (class file 55.0)",
		},
		{
			name:             "OutOfMemory heap space",
			input:            "java.lang.OutOfMemoryError: Java heap space\n\tat net.minecraft.world.level.Level.getBlockState(Level.java:234)",
			expectedCategory: CategoryOOM,
			expectedCulprit:  "JVM Heap Exhaustion",
		},
		{
			name:             "OutOfMemory GC overhead",
			input:            "java.lang.OutOfMemoryError: GC overhead limit exceeded",
			expectedCategory: CategoryOOM,
			expectedCulprit:  "JVM Heap Exhaustion",
		},
		{
			name:             "Port conflict with extracted port",
			input:            "**** FAILED TO BIND TO PORT!\n[20:10:00 WARN]: The exception was: java.net.BindException: Address already in use: bind to port 25565",
			expectedCategory: CategoryPortConflict,
			expectedCulprit:  "Port 25565 in use",
		},
		{
			name:             "Port conflict generic",
			input:            "FAILED TO BIND TO PORT! Perhaps a server is already running on that port?",
			expectedCategory: CategoryPortConflict,
			expectedCulprit:  "Port 25565 in use",
		},
		{
			name:             "Disk full error",
			input:            "java.io.IOException: No space left on device\n\tat java.io.FileOutputStream.writeBytes(Native Method)",
			expectedCategory: CategoryDiskFull,
			expectedCulprit:  "Host Storage Volume",
		},
		{
			name:             "Corrupted chunk with region file",
			input:            "net.minecraft.world.level.chunk.storage.RegionFileException: Chunk file at r.0.1.mca is corrupt",
			expectedCategory: CategoryCorruptedChunk,
			expectedCulprit:  "Region File: r.0.1.mca",
		},
		{
			name:             "Corrupted chunk with chunk coords",
			input:            "Corrupt chunk detected at chunk -12, 45! Failed to load chunk",
			expectedCategory: CategoryCorruptedChunk,
			expectedCulprit:  "Chunk [-12, 45]",
		},
		{
			name:             "Corrupted chunk generic NBT mismatch",
			input:            "java.io.StreamCorruptedException: NBT tag type mismatch loading entity",
			expectedCategory: CategoryCorruptedChunk,
			expectedCulprit:  "World Chunk / NBT Data",
		},
		{
			name:             "Watchdog timeout with ticking entity",
			input:            "--- DO NOT REPORT THIS TO PAPER - THIS IS NOT A BUG OR A CRASH ---\nServer Hang Watchdog\nA single server tick took 60.00 seconds (should be at most 0.05)\nConsidering it to be crashed, server will also allocate a crash report\nTicking entity: minecraft:zombie['Zombie'/123, l='ServerLevel[world]', x=10.5, y=64.0, z=-25.3]",
			expectedCategory: CategoryWatchdogTimeout,
			expectedCulprit:  "Ticking Entity: minecraft:zombie['Zombie'/123, l='ServerLevel[world]', x=10.5, y=64.0, z=-25.3]",
		},
		{
			name:             "Watchdog timeout with ticking block entity",
			input:            "Server Hang Watchdog\nA single server tick took 62.15 seconds\nTicking block entity: minecraft:furnace [100, 60, -200]",
			expectedCategory: CategoryWatchdogTimeout,
			expectedCulprit:  "Ticking Block: minecraft:furnace [100, 60, -200]",
		},
		{
			name:             "Watchdog timeout generic",
			input:            "Server Hang Watchdog\nA single server tick took 60.01 seconds\nConsidering it to be crashed",
			expectedCategory: CategoryWatchdogTimeout,
			expectedCulprit:  "Server Hang Watchdog",
		},
		{
			name:             "Plugin could not load",
			input:            "[12:00:00 ERROR]: Could not load 'plugins/EssentialsX-2.20.0.jar' in folder 'plugins'",
			expectedCategory: CategoryPluginFailure,
			expectedCulprit:  "EssentialsX-2.20.0.jar",
		},
		{
			name:             "Plugin error enabling",
			input:            "[12:00:00 ERROR]: Error occurred while enabling Dynmap v3.4 (Is it up to date?)",
			expectedCategory: CategoryPluginFailure,
			expectedCulprit:  "Dynmap",
		},
		{
			name:             "Plugin initializing error",
			input:            "[12:00:00 ERROR]: Exception initializing plugin WorldEdit",
			expectedCategory: CategoryPluginFailure,
			expectedCulprit:  "WorldEdit",
		},
		{
			name:             "Plugin invalid exception",
			input:            "org.bukkit.plugin.InvalidPluginException: java.lang.NoClassDefFoundError: net/milkbowl/vault/economy/Economy",
			expectedCategory: CategoryPluginFailure,
			expectedCulprit:  "Invalid Plugin",
		},
		{
			name:             "Plugin general runtime error",
			input:            "org.bukkit.plugin.UnknownDependencyException: Missing dependency Vault",
			expectedCategory: CategoryPluginFailure,
			expectedCulprit:  "Plugin Subsystem",
		},
		{
			name:             "Unknown / generic crash",
			input:            "Fatal unexpected error occurred during execution\nException in thread 'main' java.lang.RuntimeException: something broke",
			expectedCategory: CategoryUnknown,
			expectedCulprit:  "Unknown / Generic Crash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := Analyze(tt.input)
			if res.Category != tt.expectedCategory {
				t.Errorf("Expected category %q, got %q", tt.expectedCategory, res.Category)
			}
			if tt.expectedCulprit != "" && res.Culprit != tt.expectedCulprit {
				t.Errorf("Expected culprit %q, got %q", tt.expectedCulprit, res.Culprit)
			}
			if res.Title == "" || res.Summary == "" || res.Recommendation == "" {
				t.Errorf("Incomplete analysis result fields: %+v", res)
			}
		})
	}
}

func TestMapClassVersionToJava(t *testing.T) {
	testCases := map[string]string{
		"65.0": "Java 21",
		"64.0": "Java 20",
		"63.0": "Java 19",
		"62.0": "Java 18",
		"61.0": "Java 17",
		"60.0": "Java 16",
		"55.0": "Java 11",
		"52.0": "Java 8",
		"70.0": "Java Class Version 70.0",
	}

	for classVer, expected := range testCases {
		got := mapClassVersionToJava(classVer)
		if got != expected {
			t.Errorf("mapClassVersionToJava(%s) = %q, expected %q", classVer, got, expected)
		}
	}
}

func TestScanCrashReports(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Directory does not exist: should return empty slice without error
	files, err := ScanCrashReports(tempDir)
	if err != nil {
		t.Fatalf("ScanCrashReports on missing crash-reports dir returned err: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("Expected 0 files, got %d", len(files))
	}

	// 2. Create crash-reports directory
	crashDir := filepath.Join(tempDir, "crash-reports")
	if err := os.MkdirAll(crashDir, 0755); err != nil {
		t.Fatalf("Failed to create crashDir: %v", err)
	}

	// 3. Write dummy files: older crash file, newer crash file, non-txt file, directory
	oldFile := filepath.Join(crashDir, "crash-2026-09-01_10.00.00-server.txt")
	newFile := filepath.Join(crashDir, "crash-2026-09-08_12.00.00-server.txt")
	ignoredFile := filepath.Join(crashDir, "other.log")
	subDir := filepath.Join(crashDir, "subfolder")

	_ = os.MkdirAll(subDir, 0755)
	_ = os.WriteFile(ignoredFile, []byte("ignored"), 0644)

	_ = os.WriteFile(oldFile, []byte("java.lang.OutOfMemoryError: Java heap space"), 0644)
	time.Sleep(10 * time.Millisecond)
	_ = os.WriteFile(newFile, []byte("FAILED TO BIND TO PORT! Address already in use: bind to port 25565"), 0644)

	// Set distinct mod times
	now := time.Now()
	_ = os.Chtimes(oldFile, now.Add(-1*time.Hour), now.Add(-1*time.Hour))
	_ = os.Chtimes(newFile, now, now)

	// 4. Scan
	results, err := ScanCrashReports(tempDir)
	if err != nil {
		t.Fatalf("ScanCrashReports failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Expected 2 crash files, got %d", len(results))
	}

	// First file must be the newest one (newFile)
	if results[0].Filename != "crash-2026-09-08_12.00.00-server.txt" {
		t.Errorf("Expected first result to be newFile, got %s", results[0].Filename)
	}
	if results[0].Analysis.Category != CategoryPortConflict {
		t.Errorf("Expected PortConflict for newest file, got %s", results[0].Analysis.Category)
	}

	if results[1].Filename != "crash-2026-09-01_10.00.00-server.txt" {
		t.Errorf("Expected second result to be oldFile, got %s", results[1].Filename)
	}
	if results[1].Analysis.Category != CategoryOOM {
		t.Errorf("Expected OutOfMemory for older file, got %s", results[1].Analysis.Category)
	}
}
