package minecraft

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	Level   int    `json:"level,omitempty"`
}

func (s *Server) GetWhiteList() ([]Player, error) {
	return s.readPlayerFile("whitelist.json")
}

func (s *Server) GetBanned() ([]Player, error) {
	return s.readPlayerFile("banned-players.json")
}

func (s *Server) GetOps() ([]Player, error) {
	return s.readPlayerFile("ops.json")
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

// validPlayerNameRegex matches valid Minecraft Java Edition usernames and Bedrock/Floodgate
// prefixed gamertags without allowing whitespace or command-injection characters.
var validPlayerNameRegex = regexp.MustCompile(`^[.*]?[a-zA-Z0-9_]{1,32}$`)

// ValidatePlayerName ensures a player username contains only safe characters.
func ValidatePlayerName(username string) error {
	if !validPlayerNameRegex.MatchString(username) {
		return errors.New("invalid player username")
	}
	return nil
}

func (s *Server) WhiteListUser(username string) error {
	if err := ValidatePlayerName(username); err != nil {
		return err
	}

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
	if err := ValidatePlayerName(username); err != nil {
		return err
	}
	if err := s.SendCommand("whitelist remove " + username); err != nil {
		return err
	}
	return nil
}

func (s *Server) BanUser(username string, reason string) error {
	if err := ValidatePlayerName(username); err != nil {
		return err
	}
	reason = strings.NewReplacer("\r", " ", "\n", " ").Replace(reason)
	if reason == "" {
		reason = "Banned by Operator"
	}
	return s.SendCommand(fmt.Sprintf("ban %s %s", username, reason))
}

func (s *Server) UnbanUser(username string) error {
	if err := ValidatePlayerName(username); err != nil {
		return err
	}
	return s.SendCommand("pardon " + username)
}

func (s *Server) OpUser(username string) error {
	if err := ValidatePlayerName(username); err != nil {
		return err
	}
	return s.SendCommand("op " + username)
}

func (s *Server) DeopUser(username string) error {
	if err := ValidatePlayerName(username); err != nil {
		return err
	}
	return s.SendCommand("deop " + username)
}
