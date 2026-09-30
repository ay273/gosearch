package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ---- Wikipedia 源（免key，百科词条检索）----
// ---- Wikipedia source (no key, encyclopedia search) ----

const wikipediaAPIURL = "https://zh.wikipedia.org/w/api.php?action=query&list=search&format=json&srlimit=%d&srsearch=%s"

type wikipediaResp struct {
	Query struct {
		Search []struct {
			Title string `json:"title"`
			Snippet string `json:"snippet"`
		} `json:"search"`
	} `json:"query"`
}

// WikipediaSource 专业源：中文维基百科词条检索，内容由百科社区审核
// WikipediaSource: professional source via Wikipedia API, content community-reviewed
type WikipediaSource struct{}

func (WikipediaSource) Name() string { return "wikipedia" }

func (WikipediaSource) Search(query string, limit int) ([]Result, error) {
	u := fmt.Sprintf(wikipediaAPIURL, limit, url.QueryEscape(query))
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

	var data wikipediaResp
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("Wikipedia JSON 解析失败: %w", err)
	}

	results := make([]Result, 0, len(data.Query.Search))
	for _, s := range data.Query.Search {
		clean := stripHTML(s.Snippet)
		results = append(results, Result{
			Title:       s.Title,
			URL:         "https://zh.wikipedia.org/wiki/" + url.PathEscape(s.Title),
			Description: truncate(clean, 220),
		})
	}
	return results, nil
}

// stripHTML 去除摘要里的 HTML 标签（Wikipedia snippet 带 <span> 等标记）
func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
