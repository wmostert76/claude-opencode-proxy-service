package proxy

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type ModelInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Created     int    `json:"created"`
	OwnedBy     string `json:"owned_by"`
}

type modelsResponse struct {
	Data []struct {
		ID      string `json:"id"`
		Created int    `json:"created"`
	} `json:"data"`
}

var (
	cachedModels   []ModelInfo
	cachedModelsAt time.Time
	modelsMu       sync.Mutex
	modelsTTL      = 5 * time.Minute
)

func FetchModels(apiKey string) ([]ModelInfo, error) {
	modelsMu.Lock()
	defer modelsMu.Unlock()

	if cachedModels != nil && time.Since(cachedModelsAt) < modelsTTL {
		return cachedModels, nil
	}

	req, err := http.NewRequest("GET", ModelsURL, nil)
	if err != nil {
		models, _ := staleOrEmpty()
		return models, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		models, _ := staleOrEmpty()
		return models, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		models, _ := staleOrEmpty()
		return models, nil
	}

	var mr modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
		models, _ := staleOrEmpty()
		return models, err
	}

	models := make([]ModelInfo, 0, len(mr.Data))
	for _, m := range mr.Data {
		models = append(models, ModelInfo{
			ID:          "claude-" + m.ID,
			DisplayName: m.ID,
			Created:     m.Created,
			OwnedBy:     "opencode-go",
		})
	}

	cachedModels = models
	cachedModelsAt = time.Now()
	return models, nil
}

func staleOrEmpty() ([]ModelInfo, error) {
	if cachedModels != nil {
		return cachedModels, nil
	}
	return nil, nil
}

func FetchModelsAtStartup(apiKey string) {
	go func() {
		FetchModels(apiKey)
	}()
}
