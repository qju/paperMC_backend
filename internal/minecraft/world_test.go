package minecraft

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorldLifecycleAndDiagnostics(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Create a Modern (26.1+) world structure
	modernWorldDir := filepath.Join(tempDir, "survival_modern")
	_ = os.MkdirAll(filepath.Join(modernWorldDir, "dimensions", "minecraft", "overworld", "region"), 0755)
	_ = os.MkdirAll(filepath.Join(modernWorldDir, "dimensions", "minecraft", "the_nether", "region"), 0755)
	_ = os.MkdirAll(filepath.Join(modernWorldDir, "dimensions", "minecraft", "the_end", "region"), 0755)
	_ = os.WriteFile(filepath.Join(modernWorldDir, "dimensions", "minecraft", "overworld", "region", "r.0.0.mca"), []byte("mock-region-data-12345"), 0644)
	createTestLevelDat(t, filepath.Join(modernWorldDir, "level.dat"), "26.2", 1, 3, 1)

	// 2. Create a Legacy world structure
	legacyWorldDir := filepath.Join(tempDir, "legacy_world")
	_ = os.MkdirAll(filepath.Join(legacyWorldDir, "region"), 0755)
	_ = os.WriteFile(filepath.Join(legacyWorldDir, "region", "r.0.0.mca"), []byte("mock-legacy-region"), 0644)
	createTestLevelDat(t, filepath.Join(legacyWorldDir, "level.dat"), "1.20.4", 0, 1, 0)
	// Add legacy sibling nether
	_ = os.MkdirAll(filepath.Join(tempDir, "legacy_world_nether", "DIM-1"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "legacy_world_nether", "DIM-1", "r.0.0.mca"), []byte("nether-data"), 0644)

	// 3. Test Inspect Modern World
	infoModern, err := InspectWorld(tempDir, "survival_modern", true)
	if err != nil {
		t.Fatalf("InspectWorld modern failed: %v", err)
	}
	if infoModern.Format != "Modern (26.1+)" {
		t.Errorf("Expected Format 'Modern (26.1+)', got '%s'", infoModern.Format)
	}
	if infoModern.MinecraftVer != "26.2" {
		t.Errorf("Expected MinecraftVer '26.2', got '%s'", infoModern.MinecraftVer)
	}
	if infoModern.GameMode != "Creative" {
		t.Errorf("Expected GameMode 'Creative', got '%s'", infoModern.GameMode)
	}
	if !infoModern.Hardcore {
		t.Errorf("Expected Hardcore true")
	}
	if infoModern.SizeBytes <= 0 {
		t.Errorf("Expected non-zero SizeBytes, got %d", infoModern.SizeBytes)
	}

	// 4. Test Inspect Legacy World
	infoLegacy, err := InspectWorld(tempDir, "legacy_world", false)
	if err != nil {
		t.Fatalf("InspectWorld legacy failed: %v", err)
	}
	if infoLegacy.Format != "Legacy" {
		t.Errorf("Expected Format 'Legacy', got '%s'", infoLegacy.Format)
	}

	// 5. Test ListWorlds
	worlds, err := ListWorlds(tempDir, "survival_modern")
	if err != nil {
		t.Fatalf("ListWorlds failed: %v", err)
	}
	if len(worlds) != 2 {
		t.Fatalf("Expected 2 worlds in list, got %d", len(worlds))
	}
	// Active world must be first
	if !worlds[0].IsActive || worlds[0].Name != "survival_modern" {
		t.Errorf("Expected active world 'survival_modern' first in list")
	}

	// 6. Test Duplicate World
	err = DuplicateWorld(tempDir, "survival_modern", "survival_backup")
	if err != nil {
		t.Fatalf("DuplicateWorld failed: %v", err)
	}
	backupInfo, err := InspectWorld(tempDir, "survival_backup", false)
	if err != nil {
		t.Fatalf("Inspect duplicated world failed: %v", err)
	}
	if backupInfo.MinecraftVer != "26.2" || backupInfo.Format != "Modern (26.1+)" {
		t.Errorf("Duplicated world metadata mismatch: %+v", backupInfo)
	}

	// 7. Test Delete World (Fail on Active)
	err = DeleteWorld(tempDir, "survival_modern", "survival_modern")
	if err == nil {
		t.Errorf("Expected error when attempting to delete active world")
	}

	// 8. Test Delete World (Success on Inactive Backup)
	err = DeleteWorld(tempDir, "survival_backup", "survival_modern")
	if err != nil {
		t.Fatalf("DeleteWorld failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "survival_backup")); !os.IsNotExist(err) {
		t.Errorf("Expected deleted directory to be gone")
	}
}
