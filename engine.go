package main

import "fmt"

// ConfidenceEngine 置信度引擎接口 —— 可替换插件点
// 通用搜索源用 RuleEngine（黑名单+反爬+交叉验证+词表）；垂直搜索源用 PassthroughEngine（由源本身管质量）。
type ConfidenceEngine interface {
	Name() string
	// Score 对每个聚簇计算置信度，返回带置信度/嫌疑/备注的结果列表
	Score(query string, clusters []*cluster, com *cooccurMatrix, keepUGC bool) []Result
}

// ---- RuleEngine：通用搜索源默认引擎 ----
type RuleEngine struct{}
func (RuleEngine) Name() string { return "rule" }

// Score 每簇：来源计数 + 平台分层 + 交叉验证置信度 + 共现矩阵软文判定
func (RuleEngine) Score(query string, clusters []*cluster, com *cooccurMatrix, keepUGC bool) []Result {
var results []Result
	// 3) 每簇：来源计数 + 平台分层 + 交叉验证置信度 + 共现矩阵软文判定
	var filteredSpam []cluster
	for ci, c := range clusters {
		// 簇内来源平台分布：取最高平台层计算基础置信度，并统计源数
		class := clsUnknown
		authorityCount, ugcCount, total := 0, 0, len(c.sources)
		var ugcHosts []string
		seenHost := map[string]bool{}
		seenUGCHost := map[string]bool{}
		var hostsInCluster []string
		nonAuthCount := 0 // 非权威/非媒体平台数（UGC + Unknown，用于首见软文矩阵判定）
		for _, s := range c.sources {
			h := hostOf(s.URL)
			if h != "" && !seenHost[h] {
				seenHost[h] = true
				hostsInCluster = append(hostsInCluster, h)
			}
			cl := classOfHost(h)
			// 路径级 UGC 专区修正（2026-09-29 红队：域级白名单吞了号体系）
			// 网易号 www.163.com/dy/、企鹅号 news.qq.com/rain/、澎湃号 /author 都是人人可发的号，
			// 入驻成本低审核宽松，是副业软文重灾区 → 强制降为 UGC，防止被一手/媒体域误判提权
			if cl != clsUGC && pathUGCZone(s.URL) {
				cl = clsUGC
			}

			// 撤回：标题含"官网"提权（2026-09-29 判断失误）
			// 原因：bing-rss 标题=网页自写 title，非搜索引擎标注；"官网"字样可被软文伪造。
			// 搜索引擎的官网角标是 UI 装饰元素，不出现在 RSS 里，此信号不可用。

			if cl == clsAuthority {
				authorityCount++

				if class != clsAuthority {
					class = clsAuthority
				}
			} else if cl == clsUGC {
				ugcCount++
				if h != "" && !seenUGCHost[h] {
					seenUGCHost[h] = true
					ugcHosts = append(ugcHosts, h)
				}
				if class == clsUnknown {
					class = clsUGC
				}
			} else if cl == clsMedia && class == clsUnknown {
				class = clsMedia
			} else {
				// Unknown 陌生平台：不计权威，计入非权威数（软文矩阵最爱用名单外域名）
				nonAuthCount++
			}
		}

		// 记录本次簇内所有平台对共现（跨查询积累矩阵）
		for i := 0; i < len(hostsInCluster); i++ {
			for j := i + 1; j < len(hostsInCluster); j++ {
				com.addPair(hostsInCluster[i], hostsInCluster[j])
			}
		}

		// 学习可信度：簇内含权威媒体 → 同簇其他域名 Cred+1（自动发现"谁总跟权威同屏"）

		// 广告软文矩阵判定：纯 UGC(无任何权威/媒体) 且 ≥2 家强关联 → 直接过滤
		// 强关联依据：共现矩阵网络化程度 ≥1.5（这些平台历史上经常结伴发同款文）
		netScore := com.networkScore(ugcHosts)
		if class == clsUGC && ugcCount >= 2 && authorityCount == 0 && netScore >= 1.5 {
			if !keepUGC {
				c.spam = true
				c.note = fmt.Sprintf("⚠️ 广告软文矩阵：%d 个UGC平台结伴发同款文（共现网络强度 %.1f），已过滤", ugcCount, netScore)
				filteredSpam = append(filteredSpam, *c)
				continue // 不进结果
			}
		}

		// 官网启发式：单源 + Unknown 类 + 裸域名根路径 + 搜索引擎排第 1。
		// 逻辑：引擎把官网首页排第一是基于全网权重/备案/引用的排序行为，不是网页能自标的
		// （标题"官网"字样可伪造，排序行为难伪造）。
		// 一手官方站不在白名单时靠它兜底；只提媒体档 0.30，不给权威 0.60（防误判）。
		// 反利用成本：软文域名要顶掉真官网抢必应第一，需真实权重，成本 >> 利润。
		base := classBase(class)
		c.conf = crossBoost(base, total)

		// ---- 反爬探测（2026-09-29 红队信号：反爬强度 = 真实运营信号）----
		// 只对 Unknown 陌生域探测。软文不会设真反爬（挡自己流量=断命脉，激励不相容），
		// 真实运营站内容是被抄袭搬运对象，必然设防 → 反爬难伪造。
		antiS, antiDesc := 0.0, ""
		if class == clsUnknown && authorityCount == 0 {
			antiS, antiDesc = probeAntiCrawl(c.rep.URL)
		}

		// ---- 权威缺席压信（2026-09-29 事件级判定，核心防线）----
		// 认知：信任不靠"来源自己是谁"（域名/标题/备案全部可伪造），
		// 而靠"这个事件在权威世界有没有回声"（不可伪造）。
		// 权威媒体不转载第三方，但会独立报道真实事件；软文矩阵/中转站的事件在权威字典里查无此人。
		// 非权威簇（UGC + Unknown）+ 权威零覆盖 → 事件无权威回声 → 置信度减半（默认不可信）
		// Unknown 必须纳入：软文站最便宜的绕过就是"用名单外的新域名"，全盘绕开所有词表/矩阵
		// 反爬修正：强反爬(≥0.5) = 真实运营站防抄袭 → 豁免；完全开放(<0.2) = 欢迎爬取 → 压更深 ×0.3
		if (class == clsUGC || class == clsUnknown) && authorityCount == 0 {
			switch {
			case antiS >= 0.6:
				// 2026-09-30 用户修正：高反爬 = 真实运营信号，但 ≠ 权威 → 提到 0.5 封顶，不给 0.6
				c.antiStrong = true
				if c.conf < 0.5 {
					c.conf = 0.5
				}
			case antiS < 0.2:
				c.conf *= 0.3
				c.authorityAbsent = true
			default:
				c.conf *= 0.5
				c.authorityAbsent = true
			}
		}
		// 首见软文矩阵压信：单簇 3+ 非权威平台齐发同款文且权威零报道 → 即使无历史共现也压档。
		// 兜底共现矩阵的冷启动盲区：新平台第一次出现时 netScore=0 会漏网，这里不用等跨查询积累
		if (class == clsUGC || class == clsUnknown) && authorityCount == 0 && nonAuthCount >= 3 {
			c.conf *= 0.6
			c.firstSeenMatrix = true
		}

		// 商业敏感查询：UGC 源整体再压档（副业/赚钱/加盟/减肥类全是批量软文）
		if class == clsUGC {
			sens := querySensitivity(query)
			c.conf *= 1 - 0.5*sens
		}

		// 反向嫌疑降权：非权威源越"为 AI/搜索引擎优化"，置信度越低
		// 权威媒体/官方源不查（它们是内容生产者不是内容农场）
		if class != clsAuthority {
			penalty, reason := reversePenalty(query, c.rep.Title, c.rep.Description)
			if penalty > 0 {
				c.conf *= 1 - 0.8*penalty
				if penalty >= 0.35 {
					c.note = fmt.Sprintf("⚠️ 反向嫌疑 %.0f%%（%s），置信度已压低 %.0f%%", penalty*100, reason, 0.8*penalty*100)
				}
			}
		}

		// 软文嫌疑：≥2个UGC来源且无权威源 → 疑似商业软文，需工商核验；
		// 未知平台(非UGC非权威)不算软文、单源不触发"多平台一致"
		c.suspicious = false
		if class == clsUGC && ugcCount >= 2 && authorityCount == 0 {
			c.suspicious = true
		}

		// 极低置信度：只有 UGC 且来源很少 → 标注不建议相信
		c.note = ""
		switch {
		case class == clsUGC && total <= 2 && c.conf < 0.15:
			c.note = "⚠️ 仅来自人人可发平台，置信度极低，不建议采信"
		case c.firstSeenMatrix:
			c.note = fmt.Sprintf("⚠️ 权威零报道 + %d 个UGC平台齐发同款文（疑似软文矩阵），置信度 %.2f", ugcCount, c.conf)
		case c.suspicious:
			c.note = fmt.Sprintf("⚠️ 疑似商业软文（%d 个 UGC 平台一致但均非权威），建议工商核验企业规模后再判定", ugcCount)
		case c.antiOpen:
			c.note = fmt.Sprintf("⚠️ 无任何反爬（%s），内容完全开放欢迎爬取，疑似为AI/搜索引擎优化，置信度再压低", antiDesc)
		case c.authorityAbsent:
			c.note = "⚠️ 权威媒体零报道（事件无权威回声），置信度已减半，建议存疑"
		case c.antiStrong:
			c.note = fmt.Sprintf("🛡️ 强反爬（%s），真实运营站，置信度 0.5（运营真实但非权威）", antiDesc)
		case class == clsAuthority:
			c.note = fmt.Sprintf("✅ 权威/官方来源 %d 家，置信度 %.2f", authorityCount, c.conf)
		}

		rep := c.rep
		rep.Confidence = c.conf
		rep.Suspicious = c.suspicious
		rep.Note = c.note
		rep.Cluster = ci + 1
		results = append(results, rep)
	}
	return results
}
