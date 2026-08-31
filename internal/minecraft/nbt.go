package minecraft

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"
)

// NBT Tag Types
const (
	tagEnd byte = iota
	tagByte
	tagShort
	tagInt
	tagLong
	tagFloat
	tagDouble
	tagByteArray
	tagString
	tagList
	tagCompound
	tagIntArray
	tagLongArray
)

// LevelMetadata contains extracted diagnostics from level.dat
type LevelMetadata struct {
	LevelName       string
	MinecraftVer    string
	DataVersion     int
	GameMode        string
	Difficulty      string
	Hardcore        bool
	LastPlayed      time.Time
	HasValidData    bool
}

// ReadLevelDat parses a GZIP-compressed level.dat file and extracts world diagnostics.
func ReadLevelDat(filePath string) (*LevelMetadata, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize gzip reader: %w", err)
	}
	defer gz.Close()

	uncompressedData, err := io.ReadAll(gz)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress level.dat: %w", err)
	}

	meta := &LevelMetadata{
		GameMode:   "Survival",
		Difficulty: "Normal",
	}

	reader := bytes.NewReader(uncompressedData)
	parsed, err := parseNBT(reader)
	if err != nil {
		return meta, nil // Return default metadata if parsing partial
	}

	dataCompound, ok := parsed["Data"].(map[string]interface{})
	if !ok {
		// Some newer versions have root compound directly or under Data
		dataCompound = parsed
	}

	meta.HasValidData = true

	if name, ok := dataCompound["LevelName"].(string); ok {
		meta.LevelName = name
	}

	if lastPlayed, ok := dataCompound["LastPlayed"].(int64); ok && lastPlayed > 0 {
		meta.LastPlayed = time.UnixMilli(lastPlayed)
	}

	if dataVer, ok := dataCompound["DataVersion"].(int32); ok {
		meta.DataVersion = int(dataVer)
	}

	if hardcore, ok := dataCompound["hardcore"].(byte); ok {
		meta.Hardcore = hardcore == 1
	}

	// Game Mode (0=Survival, 1=Creative, 2=Adventure, 3=Spectator)
	if gameType, ok := dataCompound["GameType"].(int32); ok {
		switch gameType {
		case 0:
			meta.GameMode = "Survival"
		case 1:
			meta.GameMode = "Creative"
		case 2:
			meta.GameMode = "Adventure"
		case 3:
			meta.GameMode = "Spectator"
		}
	}

	// Difficulty (0=Peaceful, 1=Easy, 2=Normal, 3=Hard)
	if difficulty, ok := dataCompound["Difficulty"].(byte); ok {
		switch difficulty {
		case 0:
			meta.Difficulty = "Peaceful"
		case 1:
			meta.Difficulty = "Easy"
		case 2:
			meta.Difficulty = "Normal"
		case 3:
			meta.Difficulty = "Hard"
		}
	}

	// Version Compound
	if versionCompound, ok := dataCompound["Version"].(map[string]interface{}); ok {
		if verName, ok := versionCompound["Name"].(string); ok {
			meta.MinecraftVer = verName
		}
		if verId, ok := versionCompound["Id"].(int32); ok && meta.DataVersion == 0 {
			meta.DataVersion = int(verId)
		}
	}

	return meta, nil
}

func parseNBT(r *bytes.Reader) (map[string]interface{}, error) {
	tagType, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	if tagType != tagCompound {
		return nil, fmt.Errorf("expected root tagCompound (10), got %d", tagType)
	}

	// Read root name
	if _, err := readNBTString(r); err != nil {
		return nil, err
	}

	return readCompoundPayload(r)
}

func readCompoundPayload(r *bytes.Reader) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for {
		tagType, err := r.ReadByte()
		if err != nil {
			return result, err
		}
		if tagType == tagEnd {
			break
		}

		name, err := readNBTString(r)
		if err != nil {
			return result, err
		}

		val, err := readTagPayload(r, tagType)
		if err != nil {
			return result, err
		}

		result[name] = val
	}

	return result, nil
}

func readTagPayload(r *bytes.Reader, tagType byte) (interface{}, error) {
	switch tagType {
	case tagByte:
		return r.ReadByte()
	case tagShort:
		var v int16
		err := binary.Read(r, binary.BigEndian, &v)
		return v, err
	case tagInt:
		var v int32
		err := binary.Read(r, binary.BigEndian, &v)
		return v, err
	case tagLong:
		var v int64
		err := binary.Read(r, binary.BigEndian, &v)
		return v, err
	case tagFloat:
		var v float32
		err := binary.Read(r, binary.BigEndian, &v)
		return v, err
	case tagDouble:
		var v float64
		err := binary.Read(r, binary.BigEndian, &v)
		return v, err
	case tagByteArray:
		var length int32
		if err := binary.Read(r, binary.BigEndian, &length); err != nil {
			return nil, err
		}
		buf := make([]byte, length)
		_, err := r.Read(buf)
		return buf, err
	case tagString:
		return readNBTString(r)
	case tagList:
		elemType, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		var length int32
		if err := binary.Read(r, binary.BigEndian, &length); err != nil {
			return nil, err
		}
		list := make([]interface{}, 0, length)
		for i := int32(0); i < length; i++ {
			elem, err := readTagPayload(r, elemType)
			if err != nil {
				return nil, err
			}
			list = append(list, elem)
		}
		return list, nil
	case tagCompound:
		return readCompoundPayload(r)
	case tagIntArray:
		var length int32
		if err := binary.Read(r, binary.BigEndian, &length); err != nil {
			return nil, err
		}
		arr := make([]int32, length)
		err := binary.Read(r, binary.BigEndian, &arr)
		return arr, err
	case tagLongArray:
		var length int32
		if err := binary.Read(r, binary.BigEndian, &length); err != nil {
			return nil, err
		}
		arr := make([]int64, length)
		err := binary.Read(r, binary.BigEndian, &arr)
		return arr, err
	default:
		return nil, fmt.Errorf("unknown nbt tag: %d", tagType)
	}
}

func readNBTString(r *bytes.Reader) (string, error) {
	var length uint16
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return "", err
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}
