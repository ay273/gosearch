package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ---- arXiv 源（免key，学术论文检索）----
// ---- arXiv source (no key, academic paper search) ----

const arxivAPIURL = "http://export.arxiv.org/api/query?search_query=all:%s&max_results=%d"

type arxivFeed struct {
	Entries []struct {
		Title string `xml:"title"`
		ID    string `xml:"id"`
		Sum   string `xml:"summary"`
	} `xml:"entry"`
}

// ArxivSource 学术专业源：arXiv 论文检索，质量由 arXiv 平台本身保证
// ArxivSource: academic source via arXiv API, quality guaranteed by the platform
type ArxivSource struct{}

func (ArxivSource) Name() string { return "arxiv" }

func (ArxivSource) Search(query string, limit int) ([]Result, error) {
	u := fmt.Sprintf(arxivAPIURL, url.QueryEscape(query), limit)
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

	var feed arxivFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("arXiv XML 解析失败: %w", err)
	}

	results := make([]Result, 0, len(feed.Entries))
	for _, e := range feed.Entries {
		title := strings.Join(strings.Fields(e.Title), " ")
		sum := strings.Join(strings.Fields(e.Sum), " ")
		results = append(results, Result{
			Title:       title,
			URL:         strings.TrimSpace(e.ID),
			Description: truncate(sum, 220),
			Source:      "arxiv",
		})
	}
	return results, nil
}
