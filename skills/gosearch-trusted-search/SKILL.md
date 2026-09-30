---
name: gosearch-trusted-search
description:
  Trusted search pool with confidence scoring against SEO spam / paid soft articles.
  Use this whenever you need search results with trust signals (confidence 0-1),
  especially for commercial keywords (side hustles, AI API relays, cloud phone, games)
  where paid content pollutes normal search. Supports two engines: rule (general sources,
  full anti-softarticle governance) and passthrough (vertical/professional sources,
  quality managed by the source itself). Parse `-json` output for machine-readable results.
metadata:
  openclaw:
    requires:
      bins:
        - gosearch
    emoji: "🔎"
---

# gosearch 可信搜索池

通用搜索池 + 置信度打分，压制商业软文/SEO 垃圾。

## When to Use

- 搜索结果需要信任度判断（谁是官方站、谁是软文）
- 商业关键词：副业赚钱、AI API 中转站、云手机代发、游戏攻略等软文重灾区
- 需要区分权威来源 vs 人人可发的 UGC 平台
- 垂直搜索（学术/代码/专业领域）—— 用 passthrough 引擎，由源本身管质量

## CLI Usage

```bash
gosearch "查询词"                 # 默认 rule 引擎（通用源，全量治理）
gosearch -engine passthrough "查询"  # 垂直源透传（源自己管质量）
gosearch -json "查询词"            # 机器可读 JSON 输出
gosearch -keep-ugc "查询词"       # 保留纯 UGC 多平台簇（默认过滤广告软文矩阵）
gosearch -debug "查询词"          # 治理详情
```

## 引擎选择

| 引擎 | 适用 | 行为 |
|---|---|---|
| `rule`（默认） | 通用搜索源 | 黑名单 UGC 低信 + 主域名名单 + 反爬探测 + 交叉验证 + 共现矩阵 + 词表降权 |
| `passthrough` | 垂直搜索源 | 置信度中性放行（0.5-0.6），由搜索源本身管质量，不套软文治理 |

## 置信度解读

- **0.60+**：权威/官方来源（主域名名单、多源交叉验证）
- **0.50**：强反爬豁免（运营真实但非权威，封顶）
- **0.26-0.30**：媒体/未知中性档
- **0.01-0.08**：UGC 人人可发平台 / 权威缺席（零权威回声，建议存疑）

## 判断原则（红队视角）

- 标题含"官网"字样 **不可信**（可伪造，非搜索引擎标注）
- 权威媒体零报道 = 负信号（事件无权威回声，建议存疑）
- 反爬强 ≠ 权威（运营真实只证明不是垃圾站，不证明内容可信）
- 主域名名单（mainDomains）由开发者/AI 维护，命中即高置信

## 典型场景

```bash
gosearch "ubuntu 下载"          # → cn.ubuntu.com 0.60 官方源
gosearch "副业赚钱"             # → 全 0.01 UGC/权威缺席，建议存疑
gosearch -engine passthrough "arxiv 论文标题"  # 垂直源，由源管质量
```
