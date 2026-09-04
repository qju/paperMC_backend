package minecraft

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
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

func TestReadLevelDatEdgeCases(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Non-existent file
	_, err := ReadLevelDat(filepath.Join(tempDir, "does-not-exist.dat"))
	if err == nil {
		t.Error("Expected error reading non-existent level.dat, got nil")
	}

	// 2. Corrupt / Non-GZIP file
	corruptPath := filepath.Join(tempDir, "corrupt.dat")
	_ = os.WriteFile(corruptPath, []byte("plain non-gzip string"), 0644)
	_, err = ReadLevelDat(corruptPath)
	if err == nil {
		t.Error("Expected error reading non-gzip level.dat, got nil")
	}

	// 3. GZIP with wrong root tag (tagInt instead of tagCompound)
	var badRootBuf bytes.Buffer
	badRootBuf.WriteByte(tagInt) // Wrong root tag
	_ = binary.Write(&badRootBuf, binary.BigEndian, uint16(0))
	_ = binary.Write(&badRootBuf, binary.BigEndian, int32(42))

	var gzBuf bytes.Buffer
	gzWriter := gzip.NewWriter(&gzBuf)
	_, _ = gzWriter.Write(badRootBuf.Bytes())
	_ = gzWriter.Close()

	badRootPath := filepath.Join(tempDir, "bad_root.dat")
	_ = os.WriteFile(badRootPath, gzBuf.Bytes(), 0644)
	metaBadRoot, err := ReadLevelDat(badRootPath)
	if err != nil || metaBadRoot.HasValidData {
		t.Errorf("Expected fallback meta with HasValidData=false for bad root, got: %v, %v", metaBadRoot, err)
	}

	// Verify parseNBT direct error on bad root
	_, parseErr := parseNBT(bytes.NewReader(badRootBuf.Bytes()))
	if parseErr == nil || !strings.Contains(parseErr.Error(), "expected root tagCompound") {
		t.Errorf("Expected 'expected root tagCompound' from parseNBT, got: %v", parseErr)
	}

	// 4. Creative GameMode, Hard difficulty, Hardcore true, and LastPlayed
	creativePath := filepath.Join(tempDir, "creative.dat")
	createTestLevelDatWithTimestamp(t, creativePath, "1.21.1", 1, 3, 1, 1725300000000)
	meta, err := ReadLevelDat(creativePath)
	if err != nil {
		t.Fatalf("Failed to read creative level.dat: %v", err)
	}
	if meta.GameMode != "Creative" {
		t.Errorf("Expected GameMode Creative, got %s", meta.GameMode)
	}
	if meta.Difficulty != "Hard" {
		t.Errorf("Expected Difficulty Hard, got %s", meta.Difficulty)
	}
	if !meta.Hardcore {
		t.Errorf("Expected Hardcore true, got false")
	}
	if meta.LastPlayed.IsZero() {
		t.Errorf("Expected non-zero LastPlayed timestamp")
	}

	// 5. Adventure, Spectator, and Unknown GameModes
	advPath := filepath.Join(tempDir, "adv.dat")
	createTestLevelDat(t, advPath, "1.20", 2, 0, 0)
	metaAdv, _ := ReadLevelDat(advPath)
	if metaAdv.GameMode != "Adventure" || metaAdv.Difficulty != "Peaceful" {
		t.Errorf("Expected Adventure/Peaceful, got %s/%s", metaAdv.GameMode, metaAdv.Difficulty)
	}

	specPath := filepath.Join(tempDir, "spec.dat")
	createTestLevelDat(t, specPath, "1.20", 3, 1, 0)
	metaSpec, _ := ReadLevelDat(specPath)
	if metaSpec.GameMode != "Spectator" || metaSpec.Difficulty != "Easy" {
		t.Errorf("Expected Spectator/Easy, got %s/%s", metaSpec.GameMode, metaSpec.Difficulty)
	}

	unknownPath := filepath.Join(tempDir, "unk.dat")
	createTestLevelDat(t, unknownPath, "1.20", 99, 99, 0)
	metaUnk, _ := ReadLevelDat(unknownPath)
	if metaUnk.GameMode != "Survival" || metaUnk.Difficulty != "Normal" {
		t.Errorf("Expected Survival/Normal fallback, got %s/%s", metaUnk.GameMode, metaUnk.Difficulty)
	}
}

func createTestLevelDatWithTimestamp(t *testing.T, filePath string, version string, gameType int32, difficulty byte, hardcore byte, lastPlayed int64) {
	var buf bytes.Buffer
	buf.WriteByte(tagCompound)
	_ = binary.Write(&buf, binary.BigEndian, uint16(0))

	buf.WriteByte(tagCompound)
	_ = binary.Write(&buf, binary.BigEndian, uint16(4))
	buf.WriteString("Data")

	// LastPlayed (TagLong 4)
	buf.WriteByte(tagLong)
	_ = binary.Write(&buf, binary.BigEndian, uint16(10))
	buf.WriteString("LastPlayed")
	_ = binary.Write(&buf, binary.BigEndian, lastPlayed)

	// GameType
	buf.WriteByte(tagInt)
	_ = binary.Write(&buf, binary.BigEndian, uint16(8))
	buf.WriteString("GameType")
	_ = binary.Write(&buf, binary.BigEndian, gameType)

	// Difficulty
	buf.WriteByte(tagByte)
	_ = binary.Write(&buf, binary.BigEndian, uint16(10))
	buf.WriteString("Difficulty")
	buf.WriteByte(difficulty)

	// Hardcore
	buf.WriteByte(tagByte)
	_ = binary.Write(&buf, binary.BigEndian, uint16(8))
	buf.WriteString("hardcore")
	buf.WriteByte(hardcore)

	// Version
	buf.WriteByte(tagCompound)
	_ = binary.Write(&buf, binary.BigEndian, uint16(7))
	buf.WriteString("Version")

	buf.WriteByte(tagString)
	_ = binary.Write(&buf, binary.BigEndian, uint16(4))
	buf.WriteString("Name")
	_ = binary.Write(&buf, binary.BigEndian, uint16(len(version)))
	buf.WriteString(version)
	buf.WriteByte(tagEnd)

	buf.WriteByte(tagEnd)
	buf.WriteByte(tagEnd)

	var gzBuf bytes.Buffer
	gzWriter := gzip.NewWriter(&gzBuf)
	_, _ = gzWriter.Write(buf.Bytes())
	_ = gzWriter.Close()

	if err := os.WriteFile(filePath, gzBuf.Bytes(), 0644); err != nil {
		t.Fatalf("Failed to write test level.dat: %v", err)
	}
}

func TestNBTTagPayloads(t *testing.T) {
	// Test reading every tag type
	var buf bytes.Buffer

	// Short (Tag 2)
	_ = binary.Write(&buf, binary.BigEndian, int16(1234))
	// Long (Tag 4)
	_ = binary.Write(&buf, binary.BigEndian, int64(1234567890123))
	// Float (Tag 5)
	_ = binary.Write(&buf, binary.BigEndian, float32(3.14))
	// Double (Tag 6)
	_ = binary.Write(&buf, binary.BigEndian, float64(2.71828))
	// Byte Array (Tag 7)
	_ = binary.Write(&buf, binary.BigEndian, int32(3))
	buf.Write([]byte{1, 2, 3})
	// List (Tag 9) of Strings (Tag 8)
	buf.WriteByte(tagString)
	_ = binary.Write(&buf, binary.BigEndian, int32(1))
	_ = binary.Write(&buf, binary.BigEndian, uint16(5))
	buf.WriteString("hello")
	// Int Array (Tag 11)
	_ = binary.Write(&buf, binary.BigEndian, int32(2))
	_ = binary.Write(&buf, binary.BigEndian, int32(10))
	_ = binary.Write(&buf, binary.BigEndian, int32(20))
	// Long Array (Tag 12)
	_ = binary.Write(&buf, binary.BigEndian, int32(1))
	_ = binary.Write(&buf, binary.BigEndian, int64(999999))

	r := bytes.NewReader(buf.Bytes())

	vShort, err := readTagPayload(r, tagShort)
	if err != nil || vShort.(int16) != 1234 {
		t.Errorf("tagShort failed: %v, %v", vShort, err)
	}

	vLong, err := readTagPayload(r, tagLong)
	if err != nil || vLong.(int64) != 1234567890123 {
		t.Errorf("tagLong failed: %v, %v", vLong, err)
	}

	vFloat, err := readTagPayload(r, tagFloat)
	if err != nil || vFloat.(float32) < 3.0 {
		t.Errorf("tagFloat failed: %v, %v", vFloat, err)
	}

	vDouble, err := readTagPayload(r, tagDouble)
	if err != nil || vDouble.(float64) < 2.0 {
		t.Errorf("tagDouble failed: %v, %v", vDouble, err)
	}

	vBytes, err := readTagPayload(r, tagByteArray)
	if err != nil || len(vBytes.([]byte)) != 3 {
		t.Errorf("tagByteArray failed: %v", err)
	}

	vList, err := readTagPayload(r, tagList)
	if err != nil || len(vList.([]interface{})) != 1 {
		t.Errorf("tagList failed: %v", err)
	}

	vIntArr, err := readTagPayload(r, tagIntArray)
	if err != nil || len(vIntArr.([]int32)) != 2 {
		t.Errorf("tagIntArray failed: %v", err)
	}

	vLongArr, err := readTagPayload(r, tagLongArray)
	if err != nil || len(vLongArr.([]int64)) != 1 {
		t.Errorf("tagLongArray failed: %v", err)
	}

	// Unknown tag
	_, err = readTagPayload(r, 99)
	if err == nil {
		t.Error("Expected error for unknown nbt tag, got nil")
	}
}
