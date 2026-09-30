package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// gosearch 通用搜索池：配置 → 池路由 → 多源并发聚合 → 去重/聚簇 → 置信度引擎 → 排序标注
// 架构：源（Source 接口）与池（config.json）解耦——加新源实现接口+配置登记即可，不改主流程。
// 治理边界：通用池(rule)套全量软文治理；专业池(passthrough)由源本身管质量，不套治理。
//
// gosearch search pool: config → pool routing → concurrent multi-source aggregation
// → dedup/cluster → confidence engine → sort & annotate.
// Architecture: sources (Source interface) decoupled from pools (config.json) —
// adding a source = implement interface + register in config, no main-flow changes.
// Governance: general pool (rule) applies full spam defense; professional pool
// (passthrough) trusts the source itself, no governance.

func main() {
	limit := flag.Int("limit", 10, "返回条数上限 / result limit")
	debug := flag.Bool("debug", false, "打印治理详情 / print governance detail")
	keepUGC := flag.Bool("keep-ugc", false, "保留纯UGC多平台强关联簇（默认过滤）/ keep pure-UGC strong-association clusters (filtered by default)")
	poolName := flag.String("pool", "", "搜索池: general(通用,默认) | professional(专业领域) / pool name")
	engineName := flag.String("engine", "", "置信度引擎覆盖: rule | passthrough（默认按池配置）/ engine override")
	asJSON := flag.Bool("json", false, "输出 JSON（机器可读）/ JSON output for agents")
	flag.Parse()
	query := strings.TrimSpace(strings.Join(flag.Args(), " "))
	if query == "" {
		fmt.Fprintln(os.Stderr, "用法 / usage: gosearch <查询> [-pool general|professional] [-limit N] [-json] [-debug]")
		os.Exit(2)
	}

	start := time.Now()

	// 加载配置（config.json 或内置默认）
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "配置加载失败 / config error:", err)
		os.Exit(1)
	}
	pool, err := cfg.resolvePool(*poolName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// 引擎：池配置决定；-engine 可覆盖
	engine := ""
	switch {
	case *engineName != "":
		engine = *engineName
	default:
		engine = pool.Engine
	}

	// 加载跨查询演化数据（共现矩阵 / 反爬缓存）
	com := loadCooccur()
	loadAntiCache()

	// 并发拉取池内所有源
	raw := fetchAll(pool, query, *limit)

	// 1) URL 去重
	seenURL := map[string]bool{}
	uniq := make([]Result, 0, len(raw))
	for _, r := range raw {
		k := normalizeURL(r.URL)
		if seenURL[k] {
			continue
		}
		seenURL[k] = true
		uniq = append(uniq, r)
	}

	// 2) 聚簇：90% 标题相似 或 85% 摘要重叠（堵"同文不同题"的矩阵绕过）
	clusters := []*cluster{}
	for _, r := range uniq {
		placed := false
		for _, c := range clusters {
			if bigramSim(c.rep.Title, r.Title) >= 0.90 {
				c.sources = append(c.sources, r)
				placed = true
				break
			}
			if c.rep.Description != "" && r.Description != "" && bigramSim(c.rep.Description, r.Description) >= 0.85 {
				c.sources = append(c.sources, r)
				placed = true
				break
			}
		}
		if !placed {
			c := &cluster{rep: r, sources: []Result{r}}
			clusters = append(clusters, c)
		}
	}

	// 3) 置信度引擎分发（可替换插件点）
	var confEng ConfidenceEngine
	switch engine {
	case "passthrough":
		confEng = PassthroughEngine{}
	default:
		confEng = RuleEngine{}
	}
	results := confEng.Score(query, clusters, com, *keepUGC)

	// 保存跨查询演化数据
	com.save()
	saveAntiCache()

	// 4) 按置信度降序
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Confidence > results[j].Confidence
	})

	// JSON 输出：机器可读，供 Hermes/OpenClaw 等 agent 插件解析
	if *asJSON {
		type jsonOut struct {
			Query       string   `json:"query"`
			Pool        string   `json:"pool"`
			Engine      string   `json:"engine"`
			ElapsedMS   int64    `json:"elapsed_ms"`
			RawCount    int      `json:"raw_count"`
			ResultCount int      `json:"result_count"`
			Results     []Result `json:"results"`
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(jsonOut{
			Query:       query,
			Pool:        pool.Name,
			Engine:      engine,
			ElapsedMS:   time.Since(start).Milliseconds(),
			RawCount:    len(raw),
			ResultCount: len(results),
			Results:     results,
		})
		return
	}

	// 文本输出
	fmt.Printf("%s [池=%s 引擎=%s]\n", query, pool.Name, engine)
	fmt.Println(strings.Repeat("─", 46))
	for _, r := range results {
		flagS := " "
		if r.Suspicious {
			flagS = "⚠"
		}
		fmt.Printf("%.2f %s %s\n", r.Confidence, flagS, r.Title)
		if *debug && r.Note != "" {
			fmt.Printf("     └─ %s\n", r.Note)
		}
	}
}

// fetchAll 并发拉取池内所有源，汇总结果
// fetchAll: fetch from all sources in a pool concurrently and merge.
// 注：源失败不影响其他源（软失败，记录但不中断）。
func fetchAll(pool *PoolConfig, query string, limit int) []Result {
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		all     []Result
		failMsg []string
	)
	for _, sc := range pool.Sources {
		src, err := sourceByName(sc.Name)
		if err != nil {
			fmt.Fprintln(os.Stderr, "源注册缺失 / missing source:", err)
			continue
		}
		wg.Add(1)
		go func(s Source) {
			defer wg.Done()
			res, err := s.Search(query, limit)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failMsg = append(failMsg, fmt.Sprintf("%s: %v", s.Name(), err))
				return
			}
			all = append(all, res...)
		}(src)
	}
	wg.Wait()
	for _, m := range failMsg {
		fmt.Fprintln(os.Stderr, "源失败 / source error:", m)
	}
	return all
}

// truncate 截断字符串到 n 个 rune
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
