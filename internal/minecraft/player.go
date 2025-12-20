package minecraft

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Player struct {
	UUID     string `json:"uuid"`
	UserName string `json:"name"`
	// Banned players have extra fields, but we can ignore them for the list view or add them later
	Created string `json:"created,omitempty"`
	Source  string `json:"source,omitempty"`
	Expires string `json:"expires,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

func (s *Server) GetWhitelist() ([]Player, error) {
	return s.readPlayerFile("whitelist.json")
}

func (s *Server) GetBanned() ([]Player, error) {
	return s.readPlayerFile("banned-players.json")
}

func (s *Server) readPlayerFile(filename string) ([]Player, error) {
	s.mu.Lock()
	dir := s.WorkDir
	s.mu.Unlock()

	path := filepath.Join(dir, filename)

	// 1. Check if file exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []Player{}, nil
	}

	// 2. Read bytes
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// 3. Parse JSON
	var players []Player
	if err := json.Unmarshal(data, &players); err != nil {
		return nil, err
	}
	return players, nil
}

func (s *Server) WhiteListUser(username string) error {
	uuid, err := GetUUID(username)

	if err == nil {
		s.Broadcast(fmt.Sprintf("[System] Whitelisting Java UUID: %s for %s\n", uuid, username))
		return s.SendCommand("whitelist add " + username)
	}

	xuid, err := GetXUID(username)
	if err == nil {
		s.Broadcast(fmt.Sprintf("[System] Whitelisting Xbox XUID: %s for %s\n", xuid, username))

		finalName := username
		if !strings.HasPrefix(username, FloodgatePrefix) {
			finalName = FloodgatePrefix + username
		}

		return s.SendCommand("fwhitelist add " + finalName)
	}

	s.Broadcast(fmt.Sprintf("[ERROR] User %s not found on Mojang or Xbox live: %s", username, err))
	return fmt.Errorf("user not found on Mojang or Xbox Live")
	// Failure: Neither API found a user
}
func (s *Server) RemoveWhitelist(username string) error {
	if err := s.SendCommand("whitelist remove " + username); err != nil {
		return err
	}
	return nil
}

func Ban(username string, reason string) {

}
