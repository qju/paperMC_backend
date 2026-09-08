package crash

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Categories
const (
	CategoryOOM                 = "OutOfMemory"
	CategoryPortConflict        = "PortConflict"
	CategoryWatchdogTimeout     = "WatchdogTimeout"
	CategoryPluginFailure       = "PluginFailure"
	CategoryCorruptedChunk      = "CorruptedChunk"
	CategoryJavaVersionMismatch = "JavaVersionMismatch"
	CategoryEulaUnaccepted      = "EulaUnaccepted"
	CategoryDiskFull            = "DiskFull"
	CategoryUnknown             = "Unknown"
)

// AnalysisResult holds the heuristic classification of a crash or error log.
type AnalysisResult struct {
	Category       string `json:"category"`
	Title          string `json:"title"`
	Culprit        string `json:"culprit,omitempty"`
	Summary        string `json:"summary"`
	Recommendation string `json:"recommendation"`
}

// DiscoveredCrashFile represents a crash dump found on disk in crash-reports/
type DiscoveredCrashFile struct {
	Filename string         `json:"filename"`
	Path     string         `json:"path"`
	ModTime  time.Time      `json:"mod_time"`
	Size     int64          `json:"size"`
	Content  string         `json:"content"`
	Analysis AnalysisResult `json:"analysis"`
}

// Compiled regex patterns for classification & culprit extraction
var (
	reEula = regexp.MustCompile(`(?i)(You need to agree to the EULA in order to run the server|agree to the EULA|eula\.txt)`)

	reJavaVersionMismatch = regexp.MustCompile(`(?i)(UnsupportedClassVersionError|has been compiled by a more recent version of the Java Runtime|class file version (\d+\.\d+))`)
	reClassFileVersion    = regexp.MustCompile(`(?i)class file version (\d+\.\d+)`)

	reOOM = regexp.MustCompile(`(?i)(java\.lang\.OutOfMemoryError|GC overhead limit exceeded|Java heap space|unable to create new native thread)`)

	rePortConflict = regexp.MustCompile(`(?i)(FAILED TO BIND TO PORT|Address already in use|java\.net\.BindException)`)
	rePortExtract  = regexp.MustCompile(`(?i)(?:port|bind to)\s*[:=]?\s*(\d{2,5})`)

	reWatchdog        = regexp.MustCompile(`(?i)(Server Hang Watchdog|A single server tick took .* seconds|Considering it to be crashed)`)
	reTickingEntity   = regexp.MustCompile(`(?i)Ticking entity:\s*([^\r\n]+)`)
	reTickingBlockEnt = regexp.MustCompile(`(?i)Ticking block entity:\s*([^\r\n]+)`)

	reChunkCorrupt  = regexp.MustCompile(`(?i)(Chunk file at .* is in the wrong location|Corrupt chunk|Failed to load chunk|StreamCorruptedException|NBT tag type mismatch|net\.minecraft\.world\.level\.chunk\.storage)`)
	reChunkCoords   = regexp.MustCompile(`(?i)(?:chunk|region)[^\d-]*(-?\d+)[,\s]+(-?\d+)`)
	reRegionFile    = regexp.MustCompile(`(?i)r\.-?\d+\.-?\d+\.mca`)

	reDiskFull = regexp.MustCompile(`(?i)(No space left on device|Disk quota exceeded|IOException: No space left)`)

	rePluginCouldNotLoad = regexp.MustCompile(`(?i)Could not load '(?:plugins[/\\\\])?([^']+)'`)
	rePluginEnableErr    = regexp.MustCompile(`(?i)Error occurred while enabling ([^\s(:]+)`)
	rePluginInitErr      = regexp.MustCompile(`(?i)Exception initializing plugin ([^\s(:]+)`)
	rePluginInvalid      = regexp.MustCompile(`(?i)org\.bukkit\.plugin\.InvalidPluginException:\s*([^\r\n]+)`)
	rePluginGeneral      = regexp.MustCompile(`(?i)(PluginClassLoader|org\.bukkit\.plugin\.UnknownDependencyException)`)
)

// Analyze evaluates raw log or crash file content against heuristic classifiers.
func Analyze(rawContent string) AnalysisResult {
	trimmed := strings.TrimSpace(rawContent)
	if trimmed == "" {
		return AnalysisResult{
			Category:       CategoryUnknown,
			Title:          "Empty Log",
			Culprit:        "No output recorded",
			Summary:        "The provided log content was empty or contains only whitespace.",
			Recommendation: "Check the server console output or verify that the server produced logs.",
		}
	}

	// 1. Check EULA (Top priority for new server setups)
	if reEula.MatchString(trimmed) {
		return AnalysisResult{
			Category:       CategoryEulaUnaccepted,
			Title:          "Minecraft EULA Not Accepted",
			Culprit:        "eula.txt",
			Summary:        "The server process stopped because the Mojang End User License Agreement (EULA) has not been agreed to.",
			Recommendation: "1. Open `eula.txt` in the server root folder and change `eula=false` to `eula=true`.\n2. In the web dashboard settings, accept the EULA and restart the server.",
		}
	}

	// 2. Check Java Version Mismatch
	if reJavaVersionMismatch.MatchString(trimmed) {
		culprit := "Java Runtime Version"
		if m := reClassFileVersion.FindStringSubmatch(trimmed); len(m) > 1 {
			javaVer := mapClassVersionToJava(m[1])
			culprit = fmt.Sprintf("Requires %s (class file %s)", javaVer, m[1])
		}
		return AnalysisResult{
			Category:       CategoryJavaVersionMismatch,
			Title:          "Java Runtime Version Mismatch",
			Culprit:        culprit,
			Summary:        "The server JAR or an installed plugin was compiled for a newer Java version than the Java Runtime running on this system.",
			Recommendation: "1. Upgrade your host Java Runtime to Java 21 or newer.\n2. Ensure your server startup command points to the modern Java binary.",
		}
	}

	// 3. Check Out of Memory (OOM)
	if reOOM.MatchString(trimmed) {
		return AnalysisResult{
			Category:       CategoryOOM,
			Title:          "Server Ran Out of Memory (Out of Memory)",
			Culprit:        "JVM Heap Exhaustion",
			Summary:        "The Minecraft server exhausted its allocated JVM heap space or system memory, causing the Java runtime to terminate.",
			Recommendation: "1. Go to Server Flags and increase allocated RAM (e.g. set -Xmx to at least 4GB or 6GB).\n2. Switch to the 'aikar' flags preset for optimized garbage collection.\n3. Check for memory leaks or excessive entities.",
		}
	}

	// 4. Check Port Conflict
	if rePortConflict.MatchString(trimmed) {
		culprit := "Port Conflict"
		if m := rePortExtract.FindStringSubmatch(trimmed); len(m) > 1 {
			culprit = fmt.Sprintf("Port %s in use", m[1])
		} else {
			culprit = "Port 25565 in use"
		}
		return AnalysisResult{
			Category:       CategoryPortConflict,
			Title:          "Network Port Conflict (Address Already in Use)",
			Culprit:        culprit,
			Summary:        "The server could not bind to its network port because another process or server instance is already occupying it.",
			Recommendation: "1. Check if another Minecraft server or Java process is running on this machine.\n2. In Server Properties, change 'server-port' to an available port (e.g. 25566).\n3. Terminate any orphaned background Java processes.",
		}
	}

	// 5. Check Disk Full
	if reDiskFull.MatchString(trimmed) {
		return AnalysisResult{
			Category:       CategoryDiskFull,
			Title:          "Storage Drive Full (No Space Left)",
			Culprit:        "Host Storage Volume",
			Summary:        "The server's storage volume ran out of free space, preventing writes to world data, player inventories, and logs.",
			Recommendation: "1. Free up disk space on the host server volume.\n2. Clean obsolete backups and old log archives in logs/.\n3. Ensure adequate storage headroom is allocated.",
		}
	}

	// 6. Check Corrupted Chunk / NBT
	if reChunkCorrupt.MatchString(trimmed) {
		culprit := "World Chunk / NBT Data"
		if m := reRegionFile.FindString(trimmed); m != "" {
			culprit = fmt.Sprintf("Region File: %s", m)
		} else if m := reChunkCoords.FindStringSubmatch(trimmed); len(m) > 2 {
			culprit = fmt.Sprintf("Chunk [%s, %s]", m[1], m[2])
		}
		return AnalysisResult{
			Category:       CategoryCorruptedChunk,
			Title:          "World Chunk or NBT Data Corruption",
			Culprit:        culprit,
			Summary:        "The server encountered corrupted chunk data, an invalid NBT tag, or an unreadable region file on disk.",
			Recommendation: "1. Restore the affected world dimension from a recent automated backup.\n2. Use a region editor tool (e.g., MCA Selector) to prune or repair corrupted chunks.\n3. Check disk and file system health.",
		}
	}

	// 7. Check Watchdog Timeout (Tick Loop Hang)
	if reWatchdog.MatchString(trimmed) {
		culprit := "Server Hang Watchdog"
		if m := reTickingEntity.FindStringSubmatch(trimmed); len(m) > 1 {
			culprit = fmt.Sprintf("Ticking Entity: %s", strings.TrimSpace(m[1]))
		} else if m := reTickingBlockEnt.FindStringSubmatch(trimmed); len(m) > 1 {
			culprit = fmt.Sprintf("Ticking Block: %s", strings.TrimSpace(m[1]))
		}
		return AnalysisResult{
			Category:       CategoryWatchdogTimeout,
			Title:          "Watchdog Tick Loop Timeout (Server Freeze)",
			Culprit:        culprit,
			Summary:        "The server tick loop froze or took longer than the configured max-tick-time (usually 60 seconds), causing watchdog termination.",
			Recommendation: "1. Check if complex redstone clocks, mob farms, or infinite entity loops lagged the server.\n2. In server.properties, you can temporarily increase max-tick-time to 120000 or -1 (disabled) to inspect logs.\n3. Identify and optimize plugins causing thread blocking.",
		}
	}

	// 8. Check Plugin Failure
	if m := rePluginCouldNotLoad.FindStringSubmatch(trimmed); len(m) > 1 {
		return AnalysisResult{
			Category:       CategoryPluginFailure,
			Title:          "Plugin Failed to Load",
			Culprit:        filepath.Base(m[1]),
			Summary:        fmt.Sprintf("The plugin '%s' could not be loaded due to missing dependencies or an invalid plugin description.", filepath.Base(m[1])),
			Recommendation: "1. Check if the plugin jar is corrupted or requires prerequisite plugins (e.g. Vault, ProtocolLib).\n2. Verify the plugin is compatible with this Minecraft version.\n3. Remove or update the plugin jar in plugins/.",
		}
	}
	if m := rePluginEnableErr.FindStringSubmatch(trimmed); len(m) > 1 {
		return AnalysisResult{
			Category:       CategoryPluginFailure,
			Title:          "Plugin Enablement Error",
			Culprit:        m[1],
			Summary:        fmt.Sprintf("Plugin '%s' crashed with an unhandled exception while being enabled.", m[1]),
			Recommendation: "1. Check plugin configuration files in plugins/" + m[1] + "/ for syntax errors.\n2. Verify plugin compatibility with your Paper version.\n3. Update or report the error to the plugin author.",
		}
	}
	if m := rePluginInitErr.FindStringSubmatch(trimmed); len(m) > 1 {
		return AnalysisResult{
			Category:       CategoryPluginFailure,
			Title:          "Plugin Initialization Error",
			Culprit:        m[1],
			Summary:        fmt.Sprintf("Plugin '%s' crashed during constructor initialization.", m[1]),
			Recommendation: "1. Check for missing required dependencies or Java compatibility.\n2. Update the plugin to the latest release compatible with this Minecraft version.",
		}
	}
	if m := rePluginInvalid.FindStringSubmatch(trimmed); len(m) > 1 {
		return AnalysisResult{
			Category:       CategoryPluginFailure,
			Title:          "Invalid Plugin Error",
			Culprit:        "Invalid Plugin",
			Summary:        fmt.Sprintf("A plugin could not be initialized: %s", strings.TrimSpace(m[1])),
			Recommendation: "1. Verify plugin jar integrity and required library dependencies.\n2. Remove incompatible or corrupted plugin jars.",
		}
	}
	if rePluginGeneral.MatchString(trimmed) {
		return AnalysisResult{
			Category:       CategoryPluginFailure,
			Title:          "Plugin Runtime Error",
			Culprit:        "Plugin Subsystem",
			Summary:        "A Bukkit/Spigot/Paper plugin threw an exception or had an unresolved dependency during server execution.",
			Recommendation: "1. Inspect the stack trace below to identify the failing plugin package.\n2. Check that all plugin dependencies are installed in plugins/.",
		}
	}

	// 9. Unknown / Unclassified Fallback
	return AnalysisResult{
		Category:       CategoryUnknown,
		Title:          "Unclassified Server Crash",
		Culprit:        "Unknown / Generic Crash",
		Summary:        "The server shut down unexpectedly with errors that did not match known failure signatures.",
		Recommendation: "1. Review the raw crash stack trace below for error details.\n2. Use the 'Ask AI' diagnostic assistant to analyze this stack trace with an LLM.\n3. Temporarily disable recently added plugins to isolate the root cause.",
	}
}

// mapClassVersionToJava translates Java class file version numbers to user-friendly Java versions.
func mapClassVersionToJava(classVer string) string {
	switch classVer {
	case "65.0":
		return "Java 21"
	case "64.0":
		return "Java 20"
	case "63.0":
		return "Java 19"
	case "62.0":
		return "Java 18"
	case "61.0":
		return "Java 17"
	case "60.0":
		return "Java 16"
	case "55.0":
		return "Java 11"
	case "52.0":
		return "Java 8"
	default:
		return fmt.Sprintf("Java Class Version %s", classVer)
	}
}

// ScanCrashReports looks into the server workDir/crash-reports directory,
// reading and classifying crash reports. Returns reports sorted newest first.
func ScanCrashReports(workDir string) ([]DiscoveredCrashFile, error) {
	crashDir := filepath.Join(workDir, "crash-reports")
	entries, err := os.ReadDir(crashDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []DiscoveredCrashFile{}, nil
		}
		return nil, err
	}

	var results []DiscoveredCrashFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".txt") {
			continue
		}

		fullPath := filepath.Join(crashDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Read up to 512KB of crash report content
		f, err := os.Open(fullPath)
		if err != nil {
			continue
		}
		contentBytes, err := io.ReadAll(io.LimitReader(f, 512*1024))
		f.Close()
		if err != nil {
			continue
		}

		rawContent := string(contentBytes)
		analysis := Analyze(rawContent)

		results = append(results, DiscoveredCrashFile{
			Filename: entry.Name(),
			Path:     fullPath,
			ModTime:  info.ModTime(),
			Size:     info.Size(),
			Content:  rawContent,
			Analysis: analysis,
		})
	}

	// Sort newest first
	sort.Slice(results, func(i, j int) bool {
		return results[i].ModTime.After(results[j].ModTime)
	})

	return results, nil
}
