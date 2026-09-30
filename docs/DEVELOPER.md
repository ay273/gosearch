# 开发者文档

## CLI 参数

| 参数 | 默认 | 说明 |
|---|---|---|
| `-engine` | `rule` | 置信度引擎：`rule`（通用源全量治理）／`passthrough`（垂直源透传） |
| `-json` | 关 | JSON 输出（`results[]` 含 title/url/confidence/suspicious/note） |
| `-limit` | 10 | 返回条数上限 |
| `-debug` | 关 | 打印治理详情 |
| `-keep-ugc` | 关 | 保留纯 UGC 多平台强关联簇（默认过滤广告软文矩阵） |

环境变量：

| 变量 | 默认 | 说明 |
|---|---|---|
| `GOSEARCH_DATA` | 当前目录 | 运行期演化数据目录（共现矩阵、反爬缓存） |

## JSON 输出结构

```json
{
  "query": "ubuntu 下载",
  "engine": "rule",
  "elapsed_ms": 1843,
  "results": [
    {
      "title": "下载 Ubuntu 桌面版",
      "url": "https://cn.ubuntu.com/download",
      "confidence": 0.60,
      "suspicious": false,
      "note": "主域名名单"
    }
  ]
}
```

## 置信度引擎（可替换插件）

引擎实现 [`ConfidenceEngine`](../engine.go) 接口，在 `main.go` 的引擎分发处注册一行即可：

```go
type ConfidenceEngine interface {
    Name() string
    Score(query string, clusters []*cluster, com *cooccurMatrix, keepUGC bool) []Result
}
```

内置两个实现：

| 引擎 | 适用 | 行为 |
|---|---|---|
| `RuleEngine` | 通用搜索源 | 黑名单 UGC 低信 + 主域名名单 + 反爬探测 + 交叉验证 + 共现矩阵 + 词表 |
| `PassthroughEngine` | 垂直搜索源 | 置信度中性放行，由源本身管质量 |

## 接入 OpenClaw / Hermes

仓库内含双生态技能（SKILL.md，Hermes 与 OpenClaw 共享技能格式）：

```bash
# 复制到你的技能目录（二选一或都装）
cp -r skills/gosearch-trusted-search ~/.hermes/skills/
cp -r skills/gosearch-trusted-search ~/.openclaw/skills/
```

技能文档会指导 agent：何时调用 gosearch、如何解析置信度字段、低置信结果当作"权威缺席，建议存疑"处理。

## 数据目录

运行期演化数据（共现矩阵、反爬缓存）通过 `GOSEARCH_DATA` 环境变量指定目录，默认当前目录。这些是本地学习结果，不入库、不上传。
