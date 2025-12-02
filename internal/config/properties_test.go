package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadProperties(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Define the test cases (The "Table")
	tests := []struct {
		name     string
		fileData string
		want     map[string]string
		wantErr  bool
	}{
		{
			name: "Valid File",
			fileData: `
				# This is a comment
				difficulty=hard
				max-players=20
				motd=Hello Word
			`,
			want: map[string]string{
				"difficulty":  "hard",
				"max-players": "20",
				"motd":        "Hello Word",
			},
			wantErr: false,
		},
		{
			name: "Empty Lines and Spaces",
			fileData: `
				difficulty=easy
					 
				     # This is a comment
				gamemode=survival
			`,
			want: map[string]string{
				"difficulty": "easy",
				"gamemode":   "survival",
			},
			wantErr: false,
		},
		{
			name:     "Broken Line (no equals)",
			fileData: `difficulty_easy`,
			want:     map[string]string{},
			wantErr:  false,
		},
		{
			name: "Actual File",
			fileData: `
				#Minecraft server properties
				#Mon Dec 01 05:00:43 UTC 2025
				accepts-transfers=false
				allow-flight=false
				allow-nether=true
				broadcast-console-to-ops=true
				broadcast-rcon-to-ops=true
				bug-report-link=
				debug=false
				difficulty=normal
				enable-code-of-conduct=false
				enable-command-block=false
				enable-jmx-monitoring=false
				enable-query=false
				enable-rcon=false
				enable-status=true
				enforce-secure-profile=true
				enforce-whitelist=false
				entity-broadcast-range-percentage=100
				force-gamemode=false
				function-permission-level=2
				gamemode=survival
				generate-structures=true
				generator-settings={}
				hardcore=false
				hide-online-players=false
				initial-disabled-packs=
				initial-enabled-packs=vanilla
				level-name=world
				level-seed=
				level-type=minecraft\:normal
				log-ips=true
				management-server-enabled=false
				management-server-host=localhost
				management-server-port=0
				management-server-tls-enabled=true
				management-server-tls-keystore=
				management-server-tls-keystore-password=
				max-chained-neighbor-updates=1000000
				max-players=10
				max-tick-time=60000
				max-world-size=29999984
				motd=§1To jest serwer §6Michalka\n§2Zapraszam na zabawe\!\!\!
				network-compression-threshold=256
				online-mode=true
				op-permission-level=4
				pause-when-empty-seconds=-1
				player-idle-timeout=0
				prevent-proxy-connections=false
				pvp=true
				query.port=25565
				rate-limit=0
				rcon.password=
				rcon.port=25575
				region-file-compression=deflate
				require-resource-pack=false
				resource-pack=
				resource-pack-id=
				resource-pack-prompt=
				resource-pack-sha1=
				server-ip=0.0.0.0
				server-port=25565
				simulation-distance=10
				spawn-monsters=true
				spawn-protection=16
				status-heartbeat-interval=0
				sync-chunk-writes=true
				text-filtering-config=
				text-filtering-version=0
				use-native-transport=true
				view-distance=20
				white-list=true
			`,
			want: map[string]string{
				`accepts-transfers`:                       `false`,
				`allow-flight`:                            `false`,
				`allow-nether`:                            `true`,
				`broadcast-console-to-ops`:                `true`,
				`broadcast-rcon-to-ops`:                   `true`,
				`bug-report-link`:                         ``,
				`debug`:                                   `false`,
				`difficulty`:                              `normal`,
				`enable-code-of-conduct`:                  `false`,
				`enable-command-block`:                    `false`,
				`enable-jmx-monitoring`:                   `false`,
				`enable-query`:                            `false`,
				`enable-rcon`:                             `false`,
				`enable-status`:                           `true`,
				`enforce-secure-profile`:                  `true`,
				`enforce-whitelist`:                       `false`,
				`entity-broadcast-range-percentage`:       `100`,
				`force-gamemode`:                          `false`,
				`function-permission-level`:               `2`,
				`gamemode`:                                `survival`,
				`generate-structures`:                     `true`,
				`generator-settings`:                      `{}`,
				`hardcore`:                                `false`,
				`hide-online-players`:                     `false`,
				`initial-disabled-packs`:                  ``,
				`initial-enabled-packs`:                   `vanilla`,
				`level-name`:                              `world`,
				`level-seed`:                              ``,
				`level-type`:                              `minecraft\:normal`,
				`log-ips`:                                 `true`,
				`management-server-enabled`:               `false`,
				`management-server-host`:                  `localhost`,
				`management-server-port`:                  `0`,
				`management-server-tls-enabled`:           `true`,
				`management-server-tls-keystore`:          ``,
				`management-server-tls-keystore-password`: ``,
				`max-chained-neighbor-updates`:            `1000000`,
				`max-players`:                             `10`,
				`max-tick-time`:                           `60000`,
				`max-world-size`:                          `29999984`,
				`motd`:                                    `§1To jest serwer §6Michalka\n§2Zapraszam na zabawe\!\!\!`,
				`network-compression-threshold`:           `256`,
				`online-mode`:                             `true`,
				`op-permission-level`:                     `4`,
				`pause-when-empty-seconds`:                `-1`,
				`player-idle-timeout`:                     `0`,
				`prevent-proxy-connections`:               `false`,
				`pvp`:                                     `true`,
				`query.port`:                              `25565`,
				`rate-limit`:                              `0`,
				`rcon.password`:                           ``,
				`rcon.port`:                               `25575`,
				`region-file-compression`:                 `deflate`,
				`require-resource-pack`:                   `false`,
				`resource-pack`:                           ``,
				`resource-pack-id`:                        ``,
				`resource-pack-prompt`:                    ``,
				`resource-pack-sha1`:                      ``,
				`server-ip`:                               `0.0.0.0`,
				`server-port`:                             `25565`,
				`simulation-distance`:                     `10`,
				`spawn-monsters`:                          `true`,
				`spawn-protection`:                        `16`,
				`status-heartbeat-interval`:               `0`,
				`sync-chunk-writes`:                       `true`,
				`text-filtering-config`:                   ``,
				`text-filtering-version`:                  `0`,
				`use-native-transport`:                    `true`,
				`view-distance`:                           `20`,
				`white-list`:                              `true`,
			},
			wantErr: false,
		},
	}

	// Rune tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A. Write the fake server.properties
			err := os.WriteFile(filepath.Join(tmpDir, "server.properties"), []byte(tt.fileData), 0644)
			if err != nil {
				t.Fatalf("[TEST] Failed to create fixture: %v", err)
			}

			// B. call function
			got, err := LoadProperties(tmpDir)

			// C. Check error state
			if (err != nil) != tt.wantErr {
				t.Errorf("[TEST] LoadProperties() error = %v, wantErr: %v", err, tt.wantErr)
				return
			}

			// D. Check Data matches
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("[TEST] LoadProperties() =  %v, want: %v", got, tt.want)
			}

		})
	}
}

func TestSaveProperties(t *testing.T) {
	tmpDir := t.TempDir()

	// Data to save
	input := map[string]string{
		"difficulty": "hard",
		"motd":       "Test Server",
	}

	// 1. Save it
	err := SaveProperties(tmpDir, input)
	if err != nil {
		t.Fatalf("SaveProperties() error = %v", err)
	}

	// 2. Load it back to verify
	loaded, err := LoadProperties(tmpDir)
	if err != nil {
		t.Fatalf("LoadProperties() failed to reload: %v", err)
	}

	// 3. Compare
	if !reflect.DeepEqual(loaded, input) {
		t.Errorf("Saved data mismatch.\nGot: %v\nWant: %v", loaded, input)
	}
}
