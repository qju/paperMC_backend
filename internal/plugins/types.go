package plugins

import "time"

// PluginInfo represents metadata and state of an installed plugin file.
type PluginInfo struct {
	Filename         string    `json:"filename"`
	Name             string    `json:"name"`
	Version          string    `json:"version"`
	Main             string    `json:"main,omitempty"`
	Description      string    `json:"description,omitempty"`
	Authors          []string  `json:"authors,omitempty"`
	Website          string    `json:"website,omitempty"`
	APIVersion       string    `json:"api_version,omitempty"`
	Dependencies     []string  `json:"dependencies,omitempty"`
	SoftDependencies []string  `json:"soft_dependencies,omitempty"`
	IsEnabled        bool      `json:"is_enabled"`
	SizeBytes        int64     `json:"size_bytes"`
	FormattedSize    string    `json:"formatted_size"`
	ModTime          time.Time `json:"mod_time"`
}

// RawPluginYml maps common Bukkit/Spigot/Paper plugin manifest fields.
type RawPluginYml struct {
	Name        string      `yaml:"name"`
	Version     interface{} `yaml:"version"`
	Main        string      `yaml:"main"`
	Description string      `yaml:"description"`
	Author      string      `yaml:"author"`
	Authors     interface{} `yaml:"authors"`
	Website     string      `yaml:"website"`
	APIVersion  string      `yaml:"api-version"`
	Depend      interface{} `yaml:"depend"`
	SoftDepend  interface{} `yaml:"softdepend"`
	Prefix      string      `yaml:"prefix"`
}

// GeyserStatus represents the current installed and upstream state of Geyser.
type GeyserStatus struct {
	Installed        bool      `json:"installed"`
	InstalledFile    string    `json:"installed_file,omitempty"`
	InstalledVersion string    `json:"installed_version,omitempty"`
	InstalledBuild   int       `json:"installed_build,omitempty"`
	InstalledSha256  string    `json:"installed_sha256,omitempty"`
	IsEnabled        bool      `json:"is_enabled"`
	LatestVersion    string    `json:"latest_version"`
	LatestBuild      int       `json:"latest_build"`
	LatestSha256     string    `json:"latest_sha256"`
	UpdateAvailable  bool      `json:"update_available"`
	SupportedBedrock string    `json:"supported_bedrock"`
	LatestBedrockVer string    `json:"latest_bedrock_ver"`
	RecentChanges    []string  `json:"recent_changes,omitempty"`
	ReleaseDate      time.Time `json:"release_date"`
}

// FloodgateStatus represents the current installed and upstream state of Floodgate.
type FloodgateStatus struct {
	Installed        bool      `json:"installed"`
	InstalledFile    string    `json:"installed_file,omitempty"`
	InstalledVersion string    `json:"installed_version,omitempty"`
	InstalledBuild   int       `json:"installed_build,omitempty"`
	InstalledSha256  string    `json:"installed_sha256,omitempty"`
	IsEnabled        bool      `json:"is_enabled"`
	LatestVersion    string    `json:"latest_version"`
	LatestBuild      int       `json:"latest_build"`
	LatestSha256     string    `json:"latest_sha256"`
	UpdateAvailable  bool      `json:"update_available"`
	RecentChanges    []string  `json:"recent_changes,omitempty"`
	ReleaseDate      time.Time `json:"release_date"`
}

// BedrockBridgeStatus aggregates Geyser and Floodgate status for easy monitoring.
type BedrockBridgeStatus struct {
	Geyser             GeyserStatus    `json:"geyser"`
	Floodgate          FloodgateStatus `json:"floodgate"`
	OverallStatus      string          `json:"overall_status"` // "ready", "update_available", "incomplete", "missing"
	BedrockSupportInfo string          `json:"bedrock_support_info"`
}

// ModrinthSearchResult represents search response from Modrinth API.
type ModrinthSearchResult struct {
	Hits      []ModrinthHit `json:"hits"`
	TotalHits int           `json:"total_hits"`
	Limit     int           `json:"limit"`
	Offset    int           `json:"offset"`
}

// ModrinthHit represents an individual plugin item on Modrinth.
type ModrinthHit struct {
	ProjectID    string    `json:"project_id"`
	Slug         string    `json:"slug"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Categories   []string  `json:"categories"`
	IconURL      string    `json:"icon_url"`
	Author       string    `json:"author"`
	Downloads    int       `json:"downloads"`
	Followers    int       `json:"followers"`
	DateModified time.Time `json:"date_modified"`
	LatestVer    string    `json:"latest_version"`
}

// ModrinthVersion represents a specific downloadable release version on Modrinth.
type ModrinthVersion struct {
	ID           string         `json:"id"`
	ProjectID    string         `json:"project_id"`
	Name         string         `json:"name"`
	VersionNum   string         `json:"version_number"`
	GameVersions []string       `json:"game_versions"`
	Loaders      []string       `json:"loaders"`
	Files        []ModrinthFile `json:"files"`
}

// ModrinthFile represents a downloadable asset file within a Modrinth version.
type ModrinthFile struct {
	Hashes   map[string]string `json:"hashes"`
	URL      string            `json:"url"`
	Filename string            `json:"filename"`
	Primary  bool              `json:"primary"`
	Size     int64             `json:"size"`
}
