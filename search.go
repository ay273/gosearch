package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// punctRe 标点/空白/符号统一归一化（去重与词法分析共用）
// punctRe: strip punctuation/whitespace/symbols (shared by dedup & lexical analysis)
var punctRe = regexp.MustCompile(`[\p{P}\p{S}\s]+`)

// ---- Bing RSS 源（免key，通用搜索）----
// ---- Bing RSS source (no API key, general search) ----

const bingRSSURL = "https://cn.bing.com/search?%s"

// defaultUA 通用 User-Agent（脱敏：不含任何个人/项目私有标识）
var defaultUA = "Mozilla/5.0 (X11; Linux x86_64) gosearch/1.0"

type rss struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

// BingRSSSource 通用搜索源：Bing RSS 免 key，返回 web 结果
// BingRSSSource: general web search via Bing RSS, no API key required
type BingRSSSource struct{}

func (BingRSSSource) Name() string { return "bing" }

func (BingRSSSource) Search(query string, limit int) ([]Result, error) {
	q := url.Values{}
	q.Set("q", query)
	q.Set("format", "rss")
	req, err := http.NewRequest("GET", fmt.Sprintf(bingRSSURL, q.Encode()), nil)
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

	var feed rss
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("RSS 解析失败: %w", err)
	}
	if len(feed.Channel.Items) == 0 {
		return nil, fmt.Errorf("Bing RSS 无 <item> 条目")
	}

	results := make([]Result, 0, len(feed.Channel.Items))
	for _, it := range feed.Channel.Items {
		results = append(results, Result{
			Title:       strings.TrimSpace(it.Title),
			URL:         strings.TrimSpace(it.Link),
			Description: strings.TrimSpace(it.Description),
			Source:      "bing",
		})
	}
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// ---- URL 归一化与域名提取（去重/聚类用）----
// ---- URL normalization & host extraction (dedup/cluster) ----

// normalizeURL 去掉碎片、追踪参数、www、尾斜杠，得到稳定去重键
// normalizeURL: strip fragment/trackers/www/trailing-slash for a stable dedup key
func normalizeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Fragment = ""
	q := u.Query()
	for k := range q {
		lc := strings.ToLower(k)
		if strings.HasPrefix(lc, "utm_") || lc == "spm" || lc == "ref" || lc == "source" {
			q.Del(k)
		}
	}
	u.RawQuery = q.Encode()
	u.Host = strings.TrimPrefix(strings.ToLower(u.Host), "www.")
	u.Scheme = strings.ToLower(u.Scheme)
	return strings.TrimSuffix(u.String(), "/")
}

// hostOf 取域名（小写，去 www. 前缀）
// hostOf: lowercase host without "www." prefix
func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Host), "www.")
}

// normTitle 标题归一化：小写、去标点、去站点后缀
// normTitle: lowercase, strip punctuation & site suffixes
func normTitle(t string) string {
	s := strings.ToLower(t)
	s = punctRe.ReplaceAllString(s, "")
	for _, suf := range []string{"网易", "腾讯新闻", "搜狐", "新浪", "今日头条"} {
		if strings.HasSuffix(s, suf) {
			s = strings.TrimSpace(strings.TrimSuffix(s, suf))
		}
	}
	return s
}

// bigramSim 二元组相似度（0-1）：堵"同文不同题"的矩阵绕过
// bigramSim: bigram Jaccard similarity (0-1), blocks matrix re-spun titles
func bigramSim(a, b string) float64 {
	na, nb := normTitle(a), normTitle(b)
	if len(na) < 2 || len(nb) < 2 {
		if na == nb {
			return 1.0
		}
		return 0
	}
	runesA, runesB := []rune(na), []rune(nb)
	bigA, bigB := map[string]bool{}, map[string]bool{}
	for i := 0; i < len(runesA)-1; i++ {
		bigA[string(runesA[i:i+2])] = true
	}
	for i := 0; i < len(runesB)-1; i++ {
		bigB[string(runesB[i:i+2])] = true
	}
	intersect := 0
	for k := range bigA {
		if bigB[k] {
			intersect++
		}
	}
	if len(bigA)+len(bigB) == 0 {
		return 0
	}
	return 2.0 * float64(intersect) / float64(len(bigA)+len(bigB))
}
