package api

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"

	"paperMC_backend/internal/database"
	"paperMC_backend/internal/flags"
	"paperMC_backend/internal/minecraft"
)

type FlagsStatusResponse struct {
	Configured      *database.ServerFlags `json:"configured"`
	EffectiveFlags  []string              `json:"effective_flags"`
	ActiveArgs      []string              `json:"active_args"`
	RestartRequired bool                  `json:"restart_required"`
	ServerRunning   bool                  `json:"server_running"`
}

type SaveFlagsRequest struct {
	RAM         string `json:"ram"`
	Preset      string `json:"preset"`
	CustomFlags string `json:"custom_flags"`
}

func (h *Handler) HandleGetFlags(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var configured *database.ServerFlags
	if h.store != nil {
		f, err := h.store.GetServerFlags()
		if err == nil && f != nil {
			configured = f
		}
	}
	if configured == nil {
		configured = &database.ServerFlags{
			RAM:         flags.DefaultRAM,
			Preset:      flags.PresetAikar,
			CustomFlags: "",
		}
	}

	effective := flags.GetEffectiveFlags(configured.RAM, configured.Preset, configured.CustomFlags)
	activeArgs := h.mc.GetActiveArgs()
	isRunning := h.mc.GetStatus() == minecraft.StatusRunning

	restartRequired := false
	if isRunning && len(activeArgs) > 0 {
		expectedArgs := flags.BuildJavaArgs(configured.RAM, configured.Preset, configured.CustomFlags, h.mc.JarFile)
		if !reflect.DeepEqual(activeArgs, expectedArgs) {
			restartRequired = true
		}
	}

	resp := FlagsStatusResponse{
		Configured:      configured,
		EffectiveFlags:  effective,
		ActiveArgs:      activeArgs,
		RestartRequired: restartRequired,
		ServerRunning:   isRunning,
	}

	respondWithJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleSaveFlags(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req SaveFlagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	// Validate RAM
	cleanRAM := strings.TrimSpace(req.RAM)
	if cleanRAM == "" {
		cleanRAM = flags.DefaultRAM
	}
	if err := flags.ValidateRAM(cleanRAM); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Validate preset
	cleanPreset := strings.ToLower(strings.TrimSpace(req.Preset))
	switch cleanPreset {
	case flags.PresetAikar, flags.PresetMinimal, flags.PresetNone, flags.PresetCustom:
		// Valid
	default:
		respondWithError(w, http.StatusBadRequest, "Invalid preset. Must be 'aikar', 'minimal', 'none', or 'custom'")
		return
	}

	toSave := &database.ServerFlags{
		RAM:         cleanRAM,
		Preset:      cleanPreset,
		CustomFlags: strings.TrimSpace(req.CustomFlags),
	}

	if h.store != nil {
		if err := h.store.SaveServerFlags(toSave); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to save server flags: "+err.Error())
			return
		}
	}

	// Update in-memory server RAM so next start picks it up even without store query
	h.mc.RAM = cleanRAM

	// Check restart required
	isRunning := h.mc.GetStatus() == minecraft.StatusRunning
	activeArgs := h.mc.GetActiveArgs()
	restartRequired := false
	if isRunning && len(activeArgs) > 0 {
		expectedArgs := flags.BuildJavaArgs(toSave.RAM, toSave.Preset, toSave.CustomFlags, h.mc.JarFile)
		if !reflect.DeepEqual(activeArgs, expectedArgs) {
			restartRequired = true
		}
	}

	effective := flags.GetEffectiveFlags(toSave.RAM, toSave.Preset, toSave.CustomFlags)
	resp := FlagsStatusResponse{
		Configured:      toSave,
		EffectiveFlags:  effective,
		ActiveArgs:      activeArgs,
		RestartRequired: restartRequired,
		ServerRunning:   isRunning,
	}

	respondWithJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleGetFlagPresets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ramParam := r.URL.Query().Get("ram")
	if strings.TrimSpace(ramParam) == "" {
		ramParam = flags.DefaultRAM
	}

	presets := flags.GetAvailablePresets(ramParam)
	respondWithJSON(w, http.StatusOK, presets)
}
