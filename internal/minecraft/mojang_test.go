package minecraft

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetUUID(t *testing.T) {
	origURL := MojangBaseURL
	defer func() { MojangBaseURL = origURL }()

	// 1. Success
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/profiles/minecraft/Steve" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"Steve","id":"c06f89064c8a49119c29ea1dbd1a1452"}`))
			return
		}
		if r.URL.Path == "/users/profiles/minecraft/NotFound" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Path == "/users/profiles/minecraft/NoContent" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Path == "/users/profiles/minecraft/ServerError" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.URL.Path == "/users/profiles/minecraft/BadJSON" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`not-json`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	MojangBaseURL = server.URL

	t.Run("Success", func(t *testing.T) {
		uuid, err := GetUUID("Steve")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if uuid != "c06f89064c8a49119c29ea1dbd1a1452" {
			t.Errorf("Expected c06f89064c8a49119c29ea1dbd1a1452, got %s", uuid)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := GetUUID("NotFound")
		if err == nil {
			t.Fatal("Expected error for non-existent user")
		}
	})

	t.Run("NoContent", func(t *testing.T) {
		_, err := GetUUID("NoContent")
		if err == nil {
			t.Fatal("Expected error for no content")
		}
	})

	t.Run("ServerError", func(t *testing.T) {
		_, err := GetUUID("ServerError")
		if err == nil {
			t.Fatal("Expected error for 500 server error")
		}
	})

	t.Run("BadJSON", func(t *testing.T) {
		_, err := GetUUID("BadJSON")
		if err == nil {
			t.Fatal("Expected error for bad json")
		}
	})

	t.Run("ConnectionFailure", func(t *testing.T) {
		MojangBaseURL = "http://127.0.0.1:0" // Unreachable port
		_, err := GetUUID("Steve")
		if err == nil {
			t.Fatal("Expected connection error")
		}
	})
}

func TestGetXUID(t *testing.T) {
	origURL := GeyserBaseURL
	defer func() { GeyserBaseURL = origURL }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/xbox/xuid/BedrockPlayer" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"xuid":2535412345678901}`))
			return
		}
		if r.URL.Path == "/v2/xbox/xuid/NotFound" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Path == "/v2/xbox/xuid/NoContent" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Path == "/v2/xbox/xuid/ServerError" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.URL.Path == "/v2/xbox/xuid/BadJSON" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`invalid-json`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	GeyserBaseURL = server.URL

	t.Run("Success Direct", func(t *testing.T) {
		xuid, err := GetXUID("BedrockPlayer")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if xuid != "2535412345678901" {
			t.Errorf("Expected 2535412345678901, got %s", xuid)
		}
	})

	t.Run("Success With Floodgate Prefix", func(t *testing.T) {
		xuid1, err1 := GetXUID("*BedrockPlayer")
		if err1 != nil || xuid1 != "2535412345678901" {
			t.Errorf("Failed with asterisk prefix: %v, %s", err1, xuid1)
		}

		xuid2, err2 := GetXUID(".BedrockPlayer")
		if err2 != nil || xuid2 != "2535412345678901" {
			t.Errorf("Failed with dot prefix: %v, %s", err2, xuid2)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := GetXUID("NotFound")
		if err == nil {
			t.Fatal("Expected error for non-existent gamertag")
		}
	})

	t.Run("NoContent", func(t *testing.T) {
		_, err := GetXUID("NoContent")
		if err == nil {
			t.Fatal("Expected error for no content")
		}
	})

	t.Run("ServerError", func(t *testing.T) {
		_, err := GetXUID("ServerError")
		if err == nil {
			t.Fatal("Expected error for server error")
		}
		if !strings.Contains(err.Error(), "geyser API error: 500") {
			t.Errorf("Expected 'geyser API error: 500', got: %v", err)
		}
	})

	t.Run("BadJSON", func(t *testing.T) {
		_, err := GetXUID("BadJSON")
		if err == nil {
			t.Fatal("Expected error for bad JSON")
		}
		if !strings.Contains(err.Error(), "invalid JSON") || !strings.Contains(err.Error(), "invalid-json") {
			t.Errorf("Expected error to contain 'invalid JSON' and response body snippet, got: %v", err)
		}
	})

	t.Run("ConnectionFailure", func(t *testing.T) {
		GeyserBaseURL = "http://127.0.0.1:0"
		_, err := GetXUID("BedrockPlayer")
		if err == nil {
			t.Fatal("Expected error on connection failure")
		}
	})
}
