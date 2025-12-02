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

// MojangProfile represents the response from the Mojang API for a user profile.
type MojangProfile struct {
	Name string `json:"name"`
	Id   string `json:"id"`
}

// UUIDResolver is a function that retrieves the UUID for a given username.
// It can be swapped for testing.
var UUIDResolver = func(name string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("https://api.mojang.com/users/profiles/minecraft/%s", name)
	return fetchUUID(client, url)
}

// GetUUID retrieves the Minecraft UUID for a given username using the Mojang API.
func GetUUID(name string) (string, error) {
	return UUIDResolver(name)
}

func fetchUUID(client *http.Client, url string) (string, error) {
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

	return profile.Id, nil
}

// GeyserResponse represents the response from the Geyser API for an Xbox XUID.
type GeyserResponse struct {
	XUID int64 `json:"xuid"`
}

// XUIDResolver is a function that retrieves the XUID for a given gamertag.
// It can be swapped for testing.
var XUIDResolver = func(gamerTag string) (string, error) {
	// clean name tag if users type *Notch or .Notch
	cleanTag := strings.TrimLeft(gamerTag, ".*")
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("https://api.geysermc.org/v2/xbox/xuid/%s", cleanTag)
	return fetchXUID(client, url)
}

// GetXUID retrieves the Xbox XUID for a given gamertag using the Geyser API.
func GetXUID(gamerTag string) (string, error) {
	return XUIDResolver(gamerTag)
}

func fetchXUID(client *http.Client, url string) (string, error) {
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
		// fmt.Println(string(bodyBytes)) // Removed print for cleaner logs
		return "", fmt.Errorf("invalid JSON: %w, response body: %s", err, string(bodyBytes))
	}

	return fmt.Sprintf("%d", profile.XUID), nil
}
