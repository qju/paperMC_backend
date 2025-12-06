package updater

// Endpoint: https://api.papermc.io/v2/projects/paper/versions/<version>/builds

// BuildsResponse represents the top-level JSON object
type BuildsResponse struct {
	ProjectId   string  `json:"project_id"`
	ProjectName string  `json:"project_name"`
	Version     string  `json:"version"`
	Builds      []Build `json:"builds"`
}

// Build represents a single entry in the "builds" list
type Build struct {
	Build     int       `json:"build"`
	Time      string    `json:"time"`
	Channel   string    `json:"channel"`
	Promoted  bool      `json:"promoted"`
	Changes   []Change  `json:"changes"`
	Downloads Downloads `json:"downloads"`
}

type Change struct {
	Commit  string `json:"commit"`
	Summary string `json:"summary"`
	Message string `json:"message"`
}

type Downloads struct {
	Application Application `json:"application"`
}

type Application struct {
	Name   string `json:"name"`
	Sha256 string `json:"sha256"`
}

func GetLatestBuild(version string) (int, string, error) {
	return 0, "", nil
}

func DownloadJar(version string, build int, fileName string, targetPath string) error {
	return nil
}
