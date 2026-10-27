package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type ModelInfo struct {
	ID            string
	Status        string
	ContextLength int
}

type modelsResponse struct {
	Data []struct {
		ID            string `json:"id"`
		ContextLength int    `json:"context_length"`
		Status        struct {
			Value string `json:"value"`
		} `json:"status"`
	} `json:"data"`
}

var ErrNoModels = errors.New("server reported no models")

func ListModels(ctx context.Context, base string) ([]ModelInfo, error) {
	url := strings.TrimRight(base, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GET %s: HTTP %d: %s", url, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var decoded modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("GET %s: decode: %w", url, err)
	}
	models := make([]ModelInfo, 0, len(decoded.Data))
	for _, d := range decoded.Data {
		if d.ID == "" {
			continue
		}
		models = append(models, ModelInfo{ID: d.ID, Status: d.Status.Value, ContextLength: d.ContextLength})
	}
	if len(models) == 0 {
		return nil, ErrNoModels
	}
	return models, nil
}
