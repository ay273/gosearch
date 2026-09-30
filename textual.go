package main

import (
	"fmt"
	"regexp"
	"strings"
)

// ---- 软文文本信号（词表 + AEO 嫌疑 + 反向检测）----

// ---- AEO 嫌疑检测：识别"为 AI 答案引擎优化"的内容农场（软文包装术） ----
// 认知：正常内容为人写，AEO 内容为机器写。AEO 标题特征：问句式、清单/指南词、
// 万能套话、关键词堆砌。非权威源 + 高 AEO 嫌疑 → 置信度大幅压低。
var aeoQuestionMarkers = []string{"什么是", "为什么", "如何", "怎么", "怎样", "吗?", "吗？", "？", "?",
	"要不要", "能不能", "是否会", "该怎么"}

var aeoListMarkers = []string{"指南", "终极", "一文读懂", "大全", "合集", "盘点", "攻略", "教程",
	"榜单", "最新", "top", "top10", "干货", "值得收藏", "建议收藏", "必看", "不看后悔",
	"涨知识", "颠覆认知", "一文说清", "全面解析", "深度解析", "超全"}

// aeoSuspect 标题 AEO 优化嫌疑度（0-1）：问句 +0.4，指南词 +0.25/个，套话 +0.2/个，封顶 0.9
func aeoSuspect(title string) float64 {
	t := strings.ToLower(title)
	score := 0.0
	for _, m := range aeoQuestionMarkers {
		if strings.Contains(t, m) {
			score += 0.4
			break
		}
	}
	for _, m := range aeoListMarkers {
		if strings.Contains(t, m) {
			score += 0.25
		}
	}
	if score > 0.9 {
		score = 0.9
	}
	return score
}
// ---- 反向检测层：所有"刻意迎合 AI/搜索引擎"的痕迹都是负信号 ----
// 认知：广告软文/内容农场琢磨着让 AI 和搜索引擎相信，越是这样越不该信。
// 三个可观测痕迹（零额外请求，标题+摘要即可算）：
//   1. queryStuffing 查询词堆砌（SEO 关键词密度异常）
//   2. descVagueness  摘要空洞度（无数字/日期/机构名，全是空话）
//   3. adCopyDensity  软文话术密度（赋能/助力/打造/爆款…）

// queryStuffing 查询词在标题中堆砌程度（0-1）：同一查询词在标题里反复出现 ≥3 次 = SEO 堆砌
// 校准（2026-09-29）：标题正常包含查询词是相关性信号不是堆砌，故只看重复次数
func queryStuffing(query, title string) float64 {
	q := strings.ToLower(query)
	t := strings.ToLower(title)
	if len([]rune(q)) < 2 || len([]rune(t)) < 2 {
		return 0
	}
	// 提取查询中的关键词片段（长度≥2的中文连续段，或≥4的英文词）
	re := regexp.MustCompile(`[a-z]{4,}|[\p{Han}]{2,}`)
	kws := map[string]bool{}
	for _, m := range re.FindAllString(q, -1) {
		if len([]rune(m)) >= 2 {
			kws[m] = true
		}
	}
	if len(kws) == 0 {
		return 0
	}
	// 只统计"同一查询词在标题中出现的总次数"——重复≥3次才像堆砌
	occurrences := 0
	for kw := range kws {
		occurrences += strings.Count(t, kw)
	}
	switch {
	case occurrences >= 5:
		return 0.8
	case occurrences >= 4:
		return 0.6
	case occurrences >= 3:
		return 0.4
	}
	return 0
}

// 空话/软文套话词：无事实信息含量，AEO 内容高频使用
var vagueMarkers = []string{
	"赋能", "助力", "打造", "引领", "深耕", "聚焦", "彰显", "展现",
	"全新", "重磅", "惊喜", "不容错过", "限时", "优惠", "秒杀", "爆款",
	"颠覆", "革新", "一键", "高效", "智能", "极致", "非凡", "卓越",
	"值得关注", "引发热议", "备受瞩目", "掀起浪潮", "广受好评",
}

// descVagueness 摘要空洞度（0-1）：无具体事实锚点（数字/日期/机构）且空话多 = 内容农场
func descVagueness(desc string) float64 {
	if len([]rune(desc)) < 10 {
		return 0.5 // 摘要太短也算空洞信号
	}
	// 事实锚点：数字、百分比、年月日、货币单位 → 有具体事实，空洞低
	anchorRe := regexp.MustCompile(`\d+(\.\d+)?%?|亿元|万元|万元|年月|日报道|发布|宣布|数据显示`)
	anchors := len(anchorRe.FindAllString(desc, -1))
	// 空话密度
	vague := 0
	for _, m := range vagueMarkers {
		if strings.Contains(desc, m) {
			vague++
		}
	}
	d := strings.ToLower(desc)
	if strings.Contains(d, "点击") || strings.Contains(d, "关注") || strings.Contains(d, "扫码") {
		vague += 2
	}
	score := 0.0
	if anchors == 0 {
		score += 0.5 // 没有任何事实锚点 → 高空洞
	}
	score += 0.1 * float64(vague)
	if score > 0.85 {
		score = 0.85
	}
	return score
}

// adCopyDensity 软文话术密度（0-1）：典型营销黑话（限时抢购/仅需X元/立即下单…）
var adCopyMarkers = []string{
	"仅需", "只需", "立即", "马上", "抢购", "下单", "购买", "特惠",
	"首单", "新人价", "优惠券", "立减", "直降", "5折", "低价",
	"招商", "加盟", "代理", "零门槛", "无需经验", "轻松月入", "日赚",
	"名额有限", "先到先得", "手慢无", "爆款", "热卖", "销量",
}

func adCopyDensity(desc string) float64 {
	if len([]rune(desc)) < 10 {
		return 0
	}
	hits := 0
	for _, m := range adCopyMarkers {
		if strings.Contains(desc, m) {
			hits++
		}
	}
	if hits == 0 {
		return 0
	}
	s := 0.25 * float64(hits)
	if s > 0.8 {
		s = 0.8
	}
	return s
}

// reversePenalty 综合反向嫌疑度（0-1）：加权合成。
// 权重经实测校准（2026-09-29）：
//   - aeo 标题嫌疑最重（0.45，AI 判断最依赖标题）
//   - ad 营销话术次之（0.35，命中即极强信号）
//   - stuff 堆砌（0.10，只认重复≥3次）
//   - vague 空洞（0.10，摘要太短时常误报，权重最低）
func reversePenalty(query, title, desc string) (float64, string) {
	aeo := aeoSuspect(title)
	stuff := queryStuffing(query, title)
	vague := descVagueness(desc)
	ad := adCopyDensity(desc)
	penalty := 0.45*aeo + 0.10*stuff + 0.10*vague + 0.35*ad
	// 营销话术命中≥0.6（限时/抢购/加盟/免加盟）即单独判软文，保底 0.30
	if ad >= 0.6 && penalty < 0.30 {
		penalty = 0.30
	}
	if penalty > 0.9 {
		penalty = 0.9
	}
	// 记录主嫌疑信号供展示
	reasons := []string{}
	if aeo >= 0.4 {
		reasons = append(reasons, fmt.Sprintf("AEO标题嫌疑%.0f%%", aeo*100))
	}
	if stuff >= 0.4 {
		reasons = append(reasons, fmt.Sprintf("查询词堆砌%.0f%%", stuff*100))
	}
	if vague >= 0.5 {
		reasons = append(reasons, fmt.Sprintf("摘要空洞%.0f%%", vague*100))
	}
	if ad >= 0.25 {
		reasons = append(reasons, fmt.Sprintf("营销话术%.0f%%", ad*100))
	}
	return penalty, strings.Join(reasons, "、")
}
