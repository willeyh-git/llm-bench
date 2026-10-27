package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestListModelsParsesFixture(t *testing.T) {
	fixture, err := os.ReadFile("testdata/models.json")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	}))
	defer srv.Close()

	models, err := ListModels(context.Background(), srv.URL+"/v1/")
	if err != nil {
		t.Fatal(err)
	}
	want := []ModelInfo{
		{ID: "Devstral Small 2 24B", Status: "unloaded", ContextLength: 32768},
		{ID: "Flint 40B A2.5B", Status: "loaded", ContextLength: 153600},
		{ID: "Gemma-4-12B", Status: "unloaded", ContextLength: 131072},
	}
	if len(models) != len(want) {
		t.Fatalf("got %d models, want %d", len(models), len(want))
	}
	for i := range want {
		if models[i] != want[i] {
			t.Errorf("model %d: got %+v, want %+v", i, models[i], want[i])
		}
	}
}

func TestListModelsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := ListModels(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

func TestListModelsEmptyData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer srv.Close()

	_, err := ListModels(context.Background(), srv.URL)
	if !errors.Is(err, ErrNoModels) {
		t.Fatalf("got %v, want ErrNoModels", err)
	}
}

func TestListModelsUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	_, err := ListModels(context.Background(), url)
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}
