package main

import (
	"net/url"
	"strings"
)

// ---- 平台分类表（置信度来源分层）----

// ugcPlatforms 人人能发的平台（商业软文聚集地）→ 置信度 0.08 起步
var ugcPlatforms = []string{
	"toutiao.com", "toutiaocdn.com",     // 今日头条
	"sohu.com",                            // 搜狐号（自媒体号）
	"baijiahao.baidu.com",                 // 百家号
	"zhidao.baidu.com", "baike.baidu.com", // 百度知道/百科（UGC，非官方一手）
	"reddit.com", "quora.com",                    // 国外 UGC（无中文审核，同按人人可发处理）
	"jingyan.baidu.com", "wenku.baidu.com", // 百度经验/文库（UGC）
	"tieba.baidu.com",                     // 百度贴吧
	"bilibili.com",                        // B站（UGC视频+主站混
	"dy.163.com", "mp.163.com",             // 网易号 UGC（163.com 官方域归一手源）
	"new.qq.com", "kuaibao.qq.com",         // 企鹅号/腾讯新闻号（qq.com 官方域归媒体）
	"yidianzixun.com",                     // 一点资讯
	"uc.cn", "mp.uc.cn",                   // UC大鱼号
	"jianshu.com",                         // 简书
	"xiaohongshu.com", "xhslink.com",      // 小红书
	"weibo.com",                           // 微博
	"tieba.baidu.com",                     // 贴吧
	"mp.weixin.qq.com",                    // 公众号
	"zhihu.com",                           // 知乎（专栏自媒体）
	"bilibili.com",                        // B站（用户投稿）
	"douyin.com", "iesdouyin.com",         // 抖音
	"kuaishou.com",                        // 快手
	"meipian.me",                          // 美篇
	"hujiang.com", "toutiaocloud.com",
}

// authorityPlatforms 常青权威种子（政府/央媒/代码托管/大厂，十年不变的确定性来源）
var authorityPlatforms = []string{
	"gov.cn", "xinhuanet.com", "xinhuanet.com.cn", "news.cn", // 政府+新华社
	"people.com.cn", "people.cn", // 人民日报
	"cctv.com", "cctv.com.cn", // 央视
	"chinanews.com.cn", "thepaper.cn", "caixin.com", // 中新社/澎湃/财新
	"github.com", "gitlab.com", "gitee.com", "gitcode.com", // 代码托管
	"microsoft.com", "apple.com", "openai.com", "anthropic.com",
	"tencent.com", "qq.com", "alibaba.com", "aliyun.com", "taobao.com", "bytedance.com",
	"163.com", "mihoyo.com", "huaweicloud.com", "baidu.com", "huawei.com",
	"deepseek.com", "kimi.com", "wikipedia.org", "britannica.com",
}

// mainDomains 主域名名单（2026-09-30 新增，替代被移除的白名单提权）
// 维护方：开发者/AI 自行维护（用户零维护成本）——遇到确定性官方主域直接在此追加。
// 职责：官方站普遍做反爬（反爬豁免 0.5 已能兜住），但裸奔官方站（ubuntu/kernel 等）
// 无反爬会被权威缺席压到 0.01，与软文同档。此名单专救这类确定官方主域：命中即高置信 0.60。
// 只放根域（ubuntu.com 而非 downloads.ubuntu.com）；domainMatch 自动覆盖全部子域。
// 反利用成本：要进此名单需官方身份被 AI/开发者核实，域名伪造进不了名单 → 无绕过面。
var mainDomains = []string{
	// Linux/开源发行版官方（曾被极简化移出种子导致 0.01-0.02）
	"ubuntu.com", "kernel.org", "debian.org", "linuxmint.com",
	"redhat.com", "centos.org", "archlinux.org", "fedoraproject.org",
	// 开源/技术官方主域
	"apache.org", "gnu.org", "python.org", "nodejs.org", "golang.org",
	"rust-lang.org", "nginx.org", "postgresql.org", "docker.com",
	"mozilla.org", "git-scm.com",
}

// mediaPlatforms 一般媒体/垂直门户 → 中间层 0.3 起步
var mediaPlatforms = []string{
	"36kr.com", "huxiu.com", "ifanr.com", "ithome.com",   // 科技媒体
	"runoob.com", "geeksforgeeks.org", "w3schools.com",   // 教程站（Media 档，非官方非权威）
	"stackoverflow.com",                          // 开发者专业社区（Media 档）
	"infzm.com", "eeo.com.cn", "nbd.com.cn",              // 经济观察/每日经济
	"jiemian.com", "sina.com", "sina.com.cn",             // 界面/新浪
	"phoenixnewmedia.com", "ifeng.com",                   // 凤凰
	"tencent.com", "qqnews.com", "dongqiudi.com",
	"mdjinfo.com", "cailianpress.com",                    // 财联社
	"wallstreetcn.com",                                   // 华尔街见闻
	"geekpark.net", "pingwest.com", "leiphone.com", "qbitai.com",
}
// domainMatch 精确域名/子域匹配（2026-09-29 修复 P0 伪装漏洞）
// 原 classOfHost 用 strings.Contains 匹配：cctv.com.evil.com 会误判为权威媒体！
// 前缀伪装域名成本为零（垃圾站随便注册），必须改精确匹配：
// 只认「完全相等」或「host 是 domain 的子域」——evil.cctv.com 需要 cctv 授权解析，第三方伪造不了
func domainMatch(host, domain string) bool {
	host = strings.TrimPrefix(strings.ToLower(host), "www.")
	domain = strings.TrimPrefix(strings.ToLower(domain), "www.")
	if host == domain {
		return true
	}
	return strings.HasSuffix(host, "."+domain)
}

// pathUGCZone 路径级 UGC 专区修正（2026-09-29）
// 一手/媒体域下的人人可发内容频道，按 UGC 降权。软文重灾区：入驻成本低、审核宽松。
// 已覆盖：网易号(www.163.com/dy/)、企鹅号(news.qq.com/rain/ 与 /a/)、澎湃号(/author)。
func pathUGCZone(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	h := strings.ToLower(u.Host)
	p := u.Path
	switch {
	case strings.Contains(h, "163.com") && strings.HasPrefix(p, "/dy/"):
		return true
	case h == "news.qq.com" && (strings.HasPrefix(p, "/rain/") || strings.HasPrefix(p, "/a/")):
		return true
	case strings.Contains(h, "thepaper.cn") && strings.HasPrefix(p, "/author"):
		return true
	}
	return false
}

// brandsSame 品牌同一性：去 org/dom 前缀 + 复数 s 变体归一化
// openclaws.io（dom:openclaws）vs org:openclaw → 同品牌族
func brandsSame(a, b string) bool {
	trim := func(x string) string {
		x = strings.TrimPrefix(x, "org:")
		x = strings.TrimPrefix(x, "dom:")
		if len(x) > 4 && strings.HasSuffix(x, "s") {
			return strings.TrimSuffix(x, "s")
		}
		return x
	}
	return trim(a) == trim(b)
}

func classOfHost(host string) platformClass {
	h := strings.ToLower(host)
	// 匹配顺序 = 语义优先级（2026-09-29 重构）：
	//   1. UGC 最具体子域优先（tieba.baidu.com 是贴吧 UGC，不是百度官方一手源）
	//   2. 权威媒体（新华社/政府/央媒）
	//   3. 官方一手信源（github/deepseek/tencent.com 等——先于一般媒体）
	//   4. 一般媒体/垂直门户
	for _, d := range ugcPlatforms {
		if domainMatch(h, d) {
			return clsUGC
		}
	}
	for _, d := range mainDomains {
		if domainMatch(h, d) {
			return clsAuthority
		}
	}
	for _, d := range authorityPlatforms {
		if domainMatch(h, d) {
			return clsAuthority
		}
	}
	for _, d := range mediaPlatforms {
		if domainMatch(h, d) {
			return clsMedia
		}
	}
	return clsUnknown
}
// ---- 查询主题敏感度：商业转化词 → 整体压低 UGC 可信度 ----
// 实测（2026-09-29）："副业赚钱/月入/加盟/减肥"这类查询命中结果几乎100%是软文
// 主题越商业转化，人人可发平台的内容越是批量软文，整体再压档
var commercialTopicMarkers = []string{
	"副业", "赚钱", "月入", "日赚", "兼职", "加盟", "代理", "招商",
	"减肥", "祛斑", "祛痘", "生发", "美白", "丰胸", "补肾",
	"理财", "炒股", "期货", "币", "挖矿", "挂机", "刷单",
	"贷款", "信用卡", "办证", "代开", "刷量", "涨粉", "上热门",
}

// querySensitivity 返回 0-1 的主题商业转化敏感度（越高越要防软文）
func querySensitivity(query string) float64 {
	hits := 0
	q := strings.ToLower(query)
	for _, m := range commercialTopicMarkers {
		if strings.Contains(q, m) {
			hits++
		}
	}
	if hits == 0 {
		return 0
	}
	if hits >= 3 {
		return 1.0
	}
	return 0.5 + float64(hits)*0.2 // 1个=0.7 2个=0.9
}

func classBase(c platformClass) float64 {
	switch c {
	case clsUGC:
		return 0.08
	case clsMedia:
		return 0.30
	case clsAuthority:
		return 0.60
	default:
		return 0.05
	}
}
// 交叉验证增益：多源独立报道同一事件 → 置信度提升（按用户规则）
// UGC: 0.08 + (N-1)*0.01 → 4家=0.11；权威: 0.60 + (N-1)*0.10 → 3家=0.80；上限 1.0
func crossBoost(base float64, sourceCount int) float64 {
	var step float64
	switch {
	case base >= 0.60:
		step = 0.10
	case base >= 0.30:
		step = 0.05
	default:
		step = 0.01
	}
	v := base + float64(sourceCount-1)*step
	if v > 1.0 {
		v = 1.0
	}
	return v
}
