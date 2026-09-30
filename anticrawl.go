package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// ---------- 反爬探测（2026-09-30 用户决策保留：黑名单 + 反爬 + 规则朴素体系）----------
// 认知：软文/内容农场的生存逻辑 = 最大化被爬取（被收录=曝光=转化），不会设真反爬——
// 伪装"有反爬"等于挡掉自己唯一的流量来源，成本与利益倒挂（激励不相容）。
// 真实运营站内容是被抄袭搬运的对象，必然设防。
// 应用面：只对 clsUnknown 陌生域生效（UGC 平台内容用户生成，不适用）。
// 信号分级：行为证明（爬虫UA被拒/挑战）> 配置声明（robots）> 装饰头（CDN/安全头）。
// 用户修正：高反爬 ≠ 权威 → 豁免后置信度封顶 0.5，不给 0.6。

const (
	antiCacheFile = "anticrawl.json"
	antiCacheTTL  = 7 * 24 * time.Hour
	antiSpiderUA  = "Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)"
)

type antiCrawlScore struct {
	Score float64 `json:"score"`
	TS    int64   `json:"ts"`
	Desc  string  `json:"desc"`
}

var antiCache = map[string]antiCrawlScore{}

func loadAntiCache() {
	b, err := os.ReadFile(antiCacheFile)
	if err != nil {
		return
	}
	json.Unmarshal(b, &antiCache)
}

func saveAntiCache() {
	b, _ := json.Marshal(antiCache)
	os.WriteFile(antiCacheFile, b, 0644)
}

// probeAntiCrawl 探测 URL 的反爬强度（0-1），行为信号优先。
func probeAntiCrawl(rawURL string) (float64, string) {
	h := hostOf(rawURL)
	if c, ok := antiCache[h]; ok && time.Since(time.Unix(c.TS, 0)) < antiCacheTTL {
		return c.Score, c.Desc + "（缓存）"
	}
	if h == "" {
		return 0, "无域名"
	}

	score := 0.0
	descs := []string{}
	client := &http.Client{Timeout: 4 * time.Second}
	u, err := url.Parse(rawURL)
	if err != nil {
		return 0, "URL无效"
	}

	// 1) 行为证明：4 类主流爬虫 UA 并发探测（红队两轮修复）
	// 旧实现只发 Baiduspider——软文站可专门拒它骗 0.7，同时放行 Google/Bing 保收录。
	// 加 Bingbot 是关键：bing-rss 搜得到的结果必然放行 Bingbot，软文站要骗满分必须连必应一起拒（自伤收录）。
	spiderUAs := []string{
		antiSpiderUA, // Baiduspider
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"Mozilla/5.0 (compatible; Bingbot/2.0; +http://www.bing.com/bingbot.htm)",
		"Python-urllib/3.11", // 通用采集器
	}
	blocked := 0
	firstHeader := true
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, ua := range spiderUAs {
		wg.Add(1)
		go func(ua string) {
			defer wg.Done()
			req, _ := http.NewRequest("GET", rawURL, nil)
			req.Header.Set("User-Agent", ua)
			req.Header.Set("Range", "bytes=0-0")
			resp, err := client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			body := ""
			if resp.StatusCode == 206 || resp.StatusCode == 200 {
				buf := make([]byte, 2048)
				n, _ := resp.Body.Read(buf)
				body = strings.ToLower(string(buf[:n]))
			}
			if resp.StatusCode == 403 || resp.StatusCode == 401 || resp.StatusCode == 429 || antiChallengePage(body) {
				mu.Lock()
				blocked++
				mu.Unlock()
			}
			// 装饰头（低权，可伪造）：只取第一个响应
			mu.Lock()
			if firstHeader {
				firstHeader = false
				hdr := resp.Header
				if hdr.Get("Cf-Ray") != "" || strings.Contains(strings.ToLower(hdr.Get("Server")), "cloudflare") {
					score += 0.1
					descs = append(descs, "Cloudflare")
				}
				if hdr.Get("Strict-Transport-Security") != "" || hdr.Get("Content-Security-Policy") != "" {
					score += 0.1
					descs = append(descs, "安全头")
				}
			}
			mu.Unlock()
		}(ua)
	}
	wg.Wait()
	if blocked > 0 {
		score += 0.7 * float64(blocked) / 4.0
		descs = append(descs, fmt.Sprintf("拒%d/4类爬虫", blocked))
	}

	// 2) robots.txt：配置声明（中低权）。行级精确匹配（修 bug）：
	// 旧实现 strings.Contains(rt, "disallow: /") 会把 /wp-admin/ 误判成全封锁。
	// 只在 User-agent: * 段内匹配值恰好为 "/" 的 Disallow: 行。
	robotURL := fmt.Sprintf("%s://%s/robots.txt", u.Scheme, u.Host)
	if r, err := client.Get(robotURL); err == nil {
		rb, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
		r.Body.Close()
		rt := strings.ToLower(string(rb))
		uaAll, fullBlock, crawlDelay := false, false, false
		for _, line := range strings.Split(rt, "\n") {
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "user-agent: *"):
				uaAll = true
			case strings.HasPrefix(line, "disallow:"):
				val := strings.TrimSpace(strings.TrimPrefix(line, "disallow:"))
				if uaAll && val == "/" {
					fullBlock = true
				}
			case strings.HasPrefix(line, "crawl-delay"):
				crawlDelay = true
			}
		}
		if fullBlock {
			score += 0.2
			descs = append(descs, "robots全封锁")
		} else if crawlDelay {
			score += 0.1
			descs = append(descs, "robots限速")
		}
	}

	if score > 1 {
		score = 1
	}
	if len(descs) == 0 {
		descs = append(descs, "完全开放")
	}
	antiCache[h] = antiCrawlScore{score, time.Now().Unix(), strings.Join(descs, "+")}
	return score, strings.Join(descs, "+")
}

func antiChallengePage(body string) bool {
	for _, kw := range []string{"cf-challenge", "cf-chl-", "captcha", "turnstile", "verify you are human", "just a moment", "are you human", "访问验证"} {
		if strings.Contains(body, kw) {
			return true
		}
	}
	return false
}
