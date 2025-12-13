package minecraft

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type MojangProfile struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

func GetUUID(name string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	url := fmt.Sprintf("https://api.mojang.com/users/profiles/minecraft/%s", name)

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return "", errors.New("username not found")
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("mojang API error: %d", resp.StatusCode)
	}

	var profile MojangProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return "", fmt.Errorf("invalid JSON %w", err)
	}

	return profile.ID, nil
}

type GeyserResponse struct {
	XUID int64 `json:"xuid"`
}

func GetXUID(gamerTag string) (string, error) {
	// clean name tag if users type *Notch or .Notch
	cleanTag := strings.TrimLeft(gamerTag, ".*")

	client := &http.Client{Timeout: 10 * time.Second}

	url := fmt.Sprintf("https://api.geysermc.org/v2/xbox/xuid/%s", cleanTag)

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return "", errors.New("username not found")
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("mojang API error: %d", resp.StatusCode)
	}

	var profile GeyserResponse
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		// Read the body into a byte slice to print it
		bodyBytes, _ := io.ReadAll(resp.Body)
		fmt.Println(string(bodyBytes))
		return "", fmt.Errorf("invalid JSON: %w, response body: %s", err, string(bodyBytes))
	}

	return fmt.Sprintf("%d", profile.XUID), nil
}
