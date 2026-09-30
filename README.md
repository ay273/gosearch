# gosearch

> 让你的 OpenClaw / Hermes 搜索源，不再被商业软文与肮脏的搜索结果污染。

一个带置信度评分的通用搜索池：搜索之前，先把噪音过滤掉。

## 它解决什么问题

AI 客户端（OpenClaw、Hermes 等）依赖搜索源喂结果，而通用搜索源（如 Bing RSS）里混着大量商业软文：今日头条、百家号、号体系自媒体、为 AI 优化过的话术文章……它们包装得像模像样，却是不折不扣的广告垃圾。搜索池返回这些内容，AI 就会一本正经地引用软文。

gosearch 在搜索源返回后、交给 AI 之前，给每条结果打上**置信度**，并按置信度排序，让软文沉底、官方源浮顶。

## 快速开始

```bash
# 构建（纯 Go 标准库，无第三方依赖）
go build -o gosearch .

# 通用搜索（默认 rule 引擎，全量治理）
./gosearch "linux 教程"

# 垂直搜索源（透传，不套软文治理——垂直源由源本身管质量）
./gosearch -engine passthrough "千问云"

# JSON 输出（给 OpenClaw / Hermes agent 机器可读）
./gosearch -json "ubuntu 下载"
```

输出示例：

```
ubuntu 下载
──────────────────────────────
0.60  下载 Ubuntu 桌面版 | Ubuntu 官网
0.26  菜鸟教程 Ubuntu 教程
0.02  百度百科 Ubuntu
```

## 反爬探测说明

gosearch 的 `rule` 引擎会用 `Baiduspider` / `Googlebot` / `Bingbot` / `Python-urllib` 四个标准爬虫 UA 向目标站点发起轻量探测，并解析其 `robots.txt`，用于评估该站的"反爬强度"（强反爬通常意味着真实运营而非软文矩阵）。

**用途**：仅为评估站点的反爬行为画像，用于置信度加权，不抓取、不存储页面内容。

**注意**：由于以标准爬虫 UA 发起探测，站点管理员可能在访问日志中看到来自 gosearch 的爬虫请求。若你介意这一点，可将 `anticrawl.go` 中的探测逻辑替换为纯 robots.txt 解析（不发起 UA 探测），置信度体系其余部分不受影响。

## 文档

- **开发者文档**（参数、JSON 结构、引擎扩展、技能接入）：[docs/DEVELOPER.md](docs/DEVELOPER.md)
- **双生态技能**（SKILL.md，Hermes 与 OpenClaw 共享格式）：`skills/gosearch-trusted-search/`

## 许可

[MIT](LICENSE)
