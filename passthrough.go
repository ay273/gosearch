package main

import "fmt"

// PassthroughEngine 垂直搜索源透传引擎。
// 设计决策（用户 2026-09-30）：置信度/反软文治理只作用于通用搜索源；
// 垂直搜索源（学术/代码/专利/图片等专业源）由搜索源本身管质量，
// 出现软文问题的概率相对低，不套黑名单+反爬+交叉验证治理——
// 请求打到垂直源后直接按来源分类给中性置信度，不做嫌疑判定。
type PassthroughEngine struct{}

func (PassthroughEngine) Name() string { return "passthrough" }

func (PassthroughEngine) Score(query string, clusters []*cluster, com *cooccurMatrix, keepUGC bool) []Result {
	_ = query
	_ = com
	_ = keepUGC
	results := make([]Result, 0, len(clusters))
	for _, c := range clusters {
		rep := c.rep
		class := classOfHost(hostOf(rep.URL))
		// 垂直源结果按平台分层给中性置信度（不交叉验证、不共现判定、不反爬探测）
		base := classBase(class)
		if base < 0.5 {
			base = 0.5 // 垂直源不因"人人可发"被压到软文档——源本身已过滤
		}
		rep.Confidence = base
		rep.Note = fmt.Sprintf("垂直源透传：来源 %s 由搜索源本身管质量", platformName(class))
		results = append(results, rep)
	}
	return results
}
