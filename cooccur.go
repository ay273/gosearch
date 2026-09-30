package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ---- 平台共现矩阵：识别"系统性结伴发同款文"的软文平台网络 ----

// ---- 平台共现矩阵：识别"系统性结伴发同款文"的软文平台网络 ----
// 思想：共现矩阵（co-occurrence matrix）——"一起出现"是最可靠的关联信号。
// 真正抓的不是几个UGC平台碰巧同屏，而是它们跨查询长期两两共现（网络化程度高）。

type cooccurMatrix struct {
	// Pairs[hostA][hostB] = 共现次数（对称）
	Pairs map[string]map[string]int `json:"pairs"`
	// Cred[host] = 与权威媒体同簇报道同一事件的累计次数（自动学习的"可信度"）
	// 权威媒体是内容生产者，和它们同簇 = 正规渠道转载/正经报道 → 提权
	Cred map[string]int `json:"cred"`
}

// cooccurFile 共现矩阵持久化路径：优先 $GOSEARCH_DATA，默认当前目录
func cooccurFile() string {
	if dir := os.Getenv("GOSEARCH_DATA"); dir != "" {
		return filepath.Join(dir, "cooccur.json")
	}
	return "cooccur.json"
}

func newCooccur() *cooccurMatrix {
	return &cooccurMatrix{Pairs: map[string]map[string]int{}, Cred: map[string]int{}}
}

func loadCooccur() *cooccurMatrix {
	data, err := os.ReadFile(cooccurFile())
	if err != nil {
		return newCooccur()
	}
	var m cooccurMatrix
	if err := json.Unmarshal(data, &m); err != nil || m.Pairs == nil {
		return newCooccur()
	}
	return &m
}

func (m *cooccurMatrix) save() {
	os.MkdirAll(filepath.Dir(cooccurFile()), 0o755)
	data, _ := json.MarshalIndent(m, "", "  ")
	os.WriteFile(cooccurFile(), data, 0o644)
}

// addPair 记录一次共现（对称写入）
func (m *cooccurMatrix) addPair(a, b string) {
	if a == "" || b == "" || a == b {
		return
	}
	if m.Pairs[a] == nil {
		m.Pairs[a] = map[string]int{}
	}
	if m.Pairs[b] == nil {
		m.Pairs[b] = map[string]int{}
	}
	m.Pairs[a][b]++
	m.Pairs[b][a]++
}

// pairStrength 两平台历史共现次数
func (m *cooccurMatrix) pairStrength(a, b string) int {
	return m.Pairs[a][b]
}

// networkScore 一组UGC平台的网络化程度：
// 簇内所有UGC平台对的历史共现平均次数。≥1.5 说明它们经常结伴发同款文（软文矩阵特征）
func (m *cooccurMatrix) networkScore(ugcHosts []string) float64 {
	if len(ugcHosts) < 2 {
		return 0
	}
	sum, pairs := 0, 0
	for i := 0; i < len(ugcHosts); i++ {
		for j := i + 1; j < len(ugcHosts); j++ {
			sum += m.pairStrength(ugcHosts[i], ugcHosts[j])
			pairs++
		}
	}
	if pairs == 0 {
		return 0
	}
	return float64(sum) / float64(pairs)
}
