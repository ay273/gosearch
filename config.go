package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ---- 搜索池配置（config-driven）：加池/加源不改代码 ----
// ---- Pool config (config-driven): add pools/sources without touching code ----

// SourceConfig 描述一个搜索源的注册信息
type SourceConfig struct {
	Name string `json:"name"` // 源名（对应内置实现）
	Kind string `json:"kind"` // general | professional（决定是否套软文治理）
}

// PoolConfig 描述一个搜索池（一组源的组合 + 引擎选择）
type PoolConfig struct {
	Name    string         `json:"name"`
	Engine  string         `json:"engine"` // rule | passthrough
	Sources []SourceConfig `json:"sources"`
}

// AppConfig 顶层配置
type AppConfig struct {
	Pools []PoolConfig `json:"pools"`
}

// defaultConfig 内置默认配置（无 config.json 时生效）
// 通用池 → rule 引擎（全量治理）；专业池 → passthrough 引擎（源自己管质量）
var defaultConfig = AppConfig{
	Pools: []PoolConfig{
		{
			Name:   "general",
			Engine: "rule",
			Sources: []SourceConfig{
				{Name: "bing", Kind: "general"},
			},
		},
		{
			Name:   "professional",
			Engine: "passthrough",
			Sources: []SourceConfig{
				{Name: "arxiv", Kind: "professional"},
				{Name: "wikipedia", Kind: "professional"},
				{Name: "hackernews", Kind: "professional"},
			},
		},
	},
}

// loadConfig 读取 config.json；不存在则用内置默认
// 配置查找顺序：$GOSEARCH_CONFIG → ./config.json
func loadConfig() (*AppConfig, error) {
	path := os.Getenv("GOSEARCH_CONFIG")
	if path == "" {
		path = filepath.Join(".", "config.json")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &defaultConfig, nil
		}
		return nil, err
	}
	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config 解析失败 %s: %w", path, err)
	}
	if len(cfg.Pools) == 0 {
		return nil, fmt.Errorf("config %s 未定义任何池", path)
	}
	return &cfg, nil
}

// resolvePool 按名取池；为空则取第一个
func (c *AppConfig) resolvePool(name string) (*PoolConfig, error) {
	if name == "" {
		return &c.Pools[0], nil
	}
	for i := range c.Pools {
		if c.Pools[i].Name == name {
			return &c.Pools[i], nil
		}
	}
	names := make([]string, 0, len(c.Pools))
	for _, p := range c.Pools {
		names = append(names, p.Name)
	}
	return nil, fmt.Errorf("池 %q 不存在，可用: %s", name, strings.Join(names, ", "))
}

// sourceByName 源名 → 内置实现（注册点：新源在这里加一行即可）
// sourceByName: name → builtin implementation (register new sources here)
func sourceByName(name string) (Source, error) {
	switch name {
	case "bing":
		return BingRSSSource{}, nil
	case "arxiv":
		return ArxivSource{}, nil
	case "wikipedia":
		return WikipediaSource{}, nil
	case "hackernews":
		return HackerNewsSource{}, nil
	default:
		return nil, fmt.Errorf("未知搜索源 %q", name)
	}
}

// Source 搜索源接口：任何新源实现 3 个方法即可接入
// Source interface: implement 3 methods to plug in a new source
type Source interface {
	Name() string
	Search(query string, limit int) ([]Result, error)
}
