package notion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/minitap-ai/eva/internal/config"
)

type Task struct {
	Title  string
	PageID string
}

func GetTaskTitle(ticketNumber int, cfg *config.Config) (Task, error) {
	query := map[string]interface{}{
		"filter": map[string]interface{}{
			"property": "ID",
			"unique_id": map[string]interface{}{
				"equals": ticketNumber,
			},
		},
	}

	jsonData, _ := json.Marshal(query)
	req, _ := http.NewRequest("POST", fmt.Sprintf("https://api.notion.com/v1/databases/%s/query", cfg.NotionDatabaseID), bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+cfg.NotionAPIKey)
	req.Header.Set("Notion-Version", "2022-06-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("❌ Failed to query Notion:", err)
		os.Exit(1)
	}

	defer resp.Body.Close()

	var result struct {
		Results []struct {
			ID         string `json:"id"`
			Properties struct {
				Name struct {
					Title []struct {
						Text struct {
							Content string `json:"content"`
						} `json:"text"`
					} `json:"title"`
				} `json:"Name"`
			} `json:"properties"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Println("❌ Failed to parse Notion response:", err)
		os.Exit(1)
	}

	if len(result.Results) == 0 || len(result.Results[0].Properties.Name.Title) == 0 {
		return Task{}, fmt.Errorf("task not found in Notion or missing title")
	}

	title := result.Results[0].Properties.Name.Title[0].Text.Content
	pageID := result.Results[0].ID
	return Task{Title: title, PageID: pageID}, nil
}

func SetTaskStatusToDoing(pageID string, cfg *config.Config) error {
	update := map[string]interface{}{
		"properties": map[string]interface{}{
			"Status": map[string]interface{}{
				"status": map[string]string{
					"name": "Doing",
				},
			},
		},
	}

	jsonData, _ := json.Marshal(update)
	req, _ := http.NewRequest("PATCH", fmt.Sprintf("https://api.notion.com/v1/pages/%s", pageID), bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+cfg.NotionAPIKey)
	req.Header.Set("Notion-Version", "2022-06-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("❌ Failed to update Notion status:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("notion returned non-200 status: %s", resp.Status)
	}

	return nil
}
