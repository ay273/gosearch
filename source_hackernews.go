package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// ---- Hacker News 源（免key，技术社区讨论）----
// ---- Hacker News source (no key, tech community discussions) ----

const hnAPIURL = "https://hn.algolia.com/api/v1/search?query=%s&hitsPerPage=%d"

type hnResp struct {
	Hits []struct {
		Title  string `json:"title"`
		URL    string `json:"url"`
		ObjectID string `json:"objectID"`
	} `json:"hits"`
}

// HackerNewsSource 专业源：HN 技术社区讨论，内容由社区投票把关
// HackerNewsSource: professional source via HN Algolia API, community-curated
type HackerNewsSource struct{}

func (HackerNewsSource) Name() string { return "hackernews" }

func (HackerNewsSource) Search(query string, limit int) ([]Result, error) {
	u := fmt.Sprintf(hnAPIURL, url.QueryEscape(query), limit)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", defaultUA)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data hnResp
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("HN JSON 解析失败: %w", err)
	}

	results := make([]Result, 0, len(data.Hits))
	for _, h := range data.Hits {
		if h.Title == "" {
			continue
		}
		u := h.URL
		if u == "" {
			u = "https://news.ycombinator.com/item?id=" + h.ObjectID
		}
		results = append(results, Result{
			Title:       h.Title,
			URL:         u,
			Description: "Hacker News",
			Source:      "hackernews",
		})
	}
	return results, nil
}
