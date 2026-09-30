package main

// ---- 结果与置信度结构（类型定义，零依赖）----

// ---- 结果与置信度结构 ----

type Result struct {
	Title       string     `json:"title"`
	URL         string     `json:"url"`
	Description string     `json:"description"`
	Position    int        `json:"position"`
	Published   string     `json:"published,omitempty"`
	Confidence  float64    `json:"confidence"` // 来源可信度 0-1
	Suspicious  bool       `json:"suspicious"` // 疑似软文/商业推广
	Note        string     `json:"note,omitempty"`
	Cluster     int        `json:"cluster"` // 聚簇ID，同事件同ID
}

// ---- 平台分类表（置信度来源分层） ----

// platformClass 平台类型: ugc=人人都能发 / media=一般媒体 / authority=权威媒体报社
type platformClass int

const (
	clsUnknown platformClass = iota
	clsUGC
	clsMedia
	clsAuthority
)
type cluster struct {
	rep        Result
	sources    []Result
	conf       float64
	suspicious bool
	spam       bool
	note       string
	// 2026-09-29：事件级权威回声判定
	authorityAbsent bool // 纯 UGC 簇且权威媒体零报道 → 事件无权威回声
	firstSeenMatrix bool // 3+ UGC 平台齐发同款文且权威零报道（无历史共现也压档）
	// 2026-09-29：反爬信号（只对 Unknown 域）
	antiStrong bool // 强反爬 → 豁免权威缺席压信（真实运营站防抄袭）
	antiOpen   bool // 完全开放 → 权威缺席压信更深（欢迎爬取 = 为AI/搜索优化）
}
func platformName(c platformClass) string {
	switch c {
	case clsUGC:
		return "UGC(人人可发)"
	case clsMedia:
		return "一般媒体"
	case clsAuthority:
		return "权威媒体"
	default:
		return "未知"
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
