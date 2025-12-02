package minecraft

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetUUID(t *testing.T) {
	// 1. Mock Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify path
		if r.URL.Path == "/users/profiles/minecraft/Steve" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(MojangProfile{
				Name: "Steve",
				Id:   "8667ba71b85a4004af54495a72c533f5",
			})
			return
		}
		if r.URL.Path == "/users/profiles/minecraft/Unknown" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	// 2. Swap Resolver
	originalResolver := UUIDResolver
	defer func() { UUIDResolver = originalResolver }()

	UUIDResolver = func(name string) (string, error) {
		client := ts.Client()
		url := ts.URL + "/users/profiles/minecraft/" + name
		return fetchUUID(client, url)
	}

	// 3. Run Tests
	t.Run("Valid User", func(t *testing.T) {
		uuid, err := GetUUID("Steve")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if uuid != "8667ba71b85a4004af54495a72c533f5" {
			t.Errorf("Expected UUID 8667ba71b85a4004af54495a72c533f5, got %s", uuid)
		}
	})

	t.Run("Unknown User", func(t *testing.T) {
		uuid, err := GetUUID("Unknown")
		if err == nil {
			t.Fatal("Expected error for unknown user, got nil")
		}
		if !strings.Contains(err.Error(), "username not found") {
			t.Errorf("Expected error message to contain 'username not found', got %v", err)
		}
		if uuid != "" {
			t.Errorf("Expected empty UUID, got %s", uuid)
		}
	})
}

func TestGetXUID(t *testing.T) {
	// 1. Mock Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify path
		if r.URL.Path == "/v2/xbox/xuid/BedrockPlayer" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(GeyserResponse{
				XUID: 25354234234234,
			})
			return
		}
		if r.URL.Path == "/v2/xbox/xuid/Unknown" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	// 2. Swap Resolver
	originalResolver := XUIDResolver
	defer func() { XUIDResolver = originalResolver }()

	XUIDResolver = func(tag string) (string, error) {
		client := ts.Client()
		url := ts.URL + "/v2/xbox/xuid/" + tag
		return fetchXUID(client, url)
	}

	// 3. Run Tests
	t.Run("Valid User", func(t *testing.T) {
		xuid, err := GetXUID("BedrockPlayer")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if xuid != "25354234234234" {
			t.Errorf("Expected XUID 25354234234234, got %s", xuid)
		}
	})

	t.Run("Unknown User", func(t *testing.T) {
		xuid, err := GetXUID("Unknown")
		if err == nil {
			t.Fatal("Expected error for unknown user, got nil")
		}
		if !strings.Contains(err.Error(), "username not found") {
			t.Errorf("Expected error message to contain 'username not found', got %v", err)
		}
		if xuid != "" {
			t.Errorf("Expected empty XUID, got %s", xuid)
		}
	})
}
