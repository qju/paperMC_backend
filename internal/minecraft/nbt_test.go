package minecraft

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// Helper to construct a basic binary NBT level.dat file
func createTestLevelDat(t *testing.T, filePath string, version string, gameType int32, difficulty byte, hardcore byte) {
	var buf bytes.Buffer

	// Root Compound (Tag 10)
	buf.WriteByte(tagCompound)
	// Root name (empty)
	_ = binary.Write(&buf, binary.BigEndian, uint16(0))

	// Compound "Data" (Tag 10)
	buf.WriteByte(tagCompound)
	_ = binary.Write(&buf, binary.BigEndian, uint16(4))
	buf.WriteString("Data")

	// GameType (TagInt 3)
	buf.WriteByte(tagInt)
	_ = binary.Write(&buf, binary.BigEndian, uint16(8))
	buf.WriteString("GameType")
	_ = binary.Write(&buf, binary.BigEndian, gameType)

	// Difficulty (TagByte 1)
	buf.WriteByte(tagByte)
	_ = binary.Write(&buf, binary.BigEndian, uint16(10))
	buf.WriteString("Difficulty")
	buf.WriteByte(difficulty)

	// Hardcore (TagByte 1)
	buf.WriteByte(tagByte)
	_ = binary.Write(&buf, binary.BigEndian, uint16(8))
	buf.WriteString("hardcore")
	buf.WriteByte(hardcore)

	// Version (TagCompound 10)
	buf.WriteByte(tagCompound)
	_ = binary.Write(&buf, binary.BigEndian, uint16(7))
	buf.WriteString("Version")

	// Version Name (TagString 8)
	buf.WriteByte(tagString)
	_ = binary.Write(&buf, binary.BigEndian, uint16(4))
	buf.WriteString("Name")
	_ = binary.Write(&buf, binary.BigEndian, uint16(len(version)))
	buf.WriteString(version)

	// End Version compound
	buf.WriteByte(tagEnd)

	// End Data compound
	buf.WriteByte(tagEnd)

	// End Root compound
	buf.WriteByte(tagEnd)

	// GZIP compress
	var gzBuf bytes.Buffer
	gzWriter := gzip.NewWriter(&gzBuf)
	_, _ = gzWriter.Write(buf.Bytes())
	_ = gzWriter.Close()

	if err := os.WriteFile(filePath, gzBuf.Bytes(), 0644); err != nil {
		t.Fatalf("Failed to write test level.dat: %v", err)
	}
}

func TestReadLevelDat(t *testing.T) {
	tempDir := t.TempDir()
	levelDatPath := filepath.Join(tempDir, "level.dat")

	createTestLevelDat(t, levelDatPath, "26.2", 0, 2, 0)

	meta, err := ReadLevelDat(levelDatPath)
	if err != nil {
		t.Fatalf("ReadLevelDat failed: %v", err)
	}

	if meta.MinecraftVer != "26.2" {
		t.Errorf("Expected MinecraftVer '26.2', got '%s'", meta.MinecraftVer)
	}
	if meta.GameMode != "Survival" {
		t.Errorf("Expected GameMode 'Survival', got '%s'", meta.GameMode)
	}
	if meta.Difficulty != "Normal" {
		t.Errorf("Expected Difficulty 'Normal', got '%s'", meta.Difficulty)
	}
	if meta.Hardcore {
		t.Errorf("Expected Hardcore false, got true")
	}
}
