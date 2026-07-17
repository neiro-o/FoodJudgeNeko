# Bot 用户信息与 AI 文字总结

本说明供接入本站 API 的 Bot 使用。Bot 应将已有的结构化用户数据和 AI 画像渲染为可读文本；不要自行根据昵称、头像或单条评论推断真实身份、性别、职业、年龄、健康、政治立场或人格。

## 前置条件

- 所有本文接口均受 JWT 保护，必须发送：

  ```http
  Authorization: Bearer <token>
  ```

- 下文以 `<API_BASE_URL>` 表示部署地址的 API 前缀，例如 `https://<host>/api`。
- 所有 JSON 响应都使用统一外层：`code === 0` 时读取 `data`；非 0 时向用户展示 `message`，不要把失败伪装成总结结果。
- `userId` 是用户的数字/字符串 ID，不是昵称。若只有昵称，先调用 `GET /user_detail/search_users?keyword=...` 获取候选用户，再让用户确认。

## 推荐调用顺序

1. 获取基础资料：`GET /user_detail/user_info?userId=<userId>`
2. 读取已缓存的 AI 画像：`GET /user_detail/ai_summary?userId=<userId>`
3. 仅在 AI 画像 `status` 为 `none` 或 `failed`，或 `stale` 为 `true` 时，调用 `POST /user_detail/ai_summary?userId=<userId>`。
4. 将基础资料和 AI 结果渲染成下方的文字格式。
5. 若需要展示评论原文，再按需调用 `GET /user_detail/comments?userId=<userId>&page=1&limit=10`；不要为了生成 AI 总结而抓取全部分页评论，后端会自行使用最高赞的最多 300 条样本（若内容总长度超出预算，会优先保留点赞更高的样本，实际条数可能少于 300）。

`POST` 是同步请求：后端会先返回 7 天内仍有效的缓存；缓存缺失或过期时才请求模型。生成中再次调用会得到 HTTP 409，应提示“总结生成中，请稍后重试”，不要并发重试。

## 接口

### 1. 基础资料

```http
GET <API_BASE_URL>/user_detail/user_info?userId=3566815
Authorization: Bearer <token>
```

成功时 `data` 字段：

```json
{
  "userName": "用户昵称",
  "likes": 12345,
  "replies": 678,
  "malicious": false
}
```

- `likes`：该用户全部评论收到的点赞总数。
- `replies`：该用户全部评论的回复总数。
- `malicious`：站内人工标记；仅在值为 `true` 时如实展示“已被站内标记为恶意账号”，不要扩展为违法或人格判断。
- 404 表示没有可展示昵称的非匿名评论；Bot 应提示资料不可用。

### 2. 缓存 AI 画像（不触发模型）

```http
GET <API_BASE_URL>/user_detail/ai_summary?userId=3566815
Authorization: Bearer <token>
```

### 3. 生成或刷新 AI 画像

```http
POST <API_BASE_URL>/user_detail/ai_summary?userId=3566815
Authorization: Bearer <token>
Content-Type: application/json
```

请求没有 body。成功响应的 `data` 核心字段：

```json
{
  "status": "ready",
  "result": {
    "roast": "针对可观察评论习惯的犀利点评",
    "profile": {
      "summary": "评论行为画像",
      "expressionStyle": ["表达特点"],
      "opinionTendency": ["观点模式"],
      "interactionPattern": ["互动特点"],
      "genderGuess": {
        "value": "无法从文本判断",
        "confidence": "low",
        "disclaimer": "非事实、不可用于判断真实身份"
      },
      "mbtiGuess": {
        "value": "无法从文本判断",
        "confidence": "low",
        "disclaimer": "非心理测量、不可用于判断真实人格"
      }
    },
    "evidence": [
      {
        "claim": "一项画像结论",
        "evidenceIds": ["c1"],
        "reason": "该评论样本呈现的模式"
      }
    ],
    "limitations": ["样本和统计数据的限制"]
  },
  "provider": "deepseek",
  "model": "…",
  "promptVersion": "user_profile_summary_v1",
  "generatedAt": 1752745200,
  "expiresAt": 1753350000,
  "stale": false
}
```

状态处理：

| `status` | Bot 行为 |
| --- | --- |
| `ready` | 直接渲染 `result`。`stale: true` 时可先展示旧结果，并说明“已过期，可刷新”。 |
| `none` | 调用一次 POST；若用户不希望触发生成，则说明“暂无 AI 总结”。 |
| `failed` | 显示 `lastError` 的简短失败提示，可由用户决定重试 POST。 |

AI 总结来自后端按点赞排序的评论样本，样本会去除用户 ID、昵称、头像和媒体 URL，并由后端进行结构校验。因此 Bot 不需要另行调用模型或拼接提示词。

## Bot 输出规范

`roast`、`summary` 和特征标签都只是“评论行为画像”，不是关于真实身份的事实。默认不输出 `genderGuess`、`mbtiGuess`；用户明确要求娱乐性内容时才可附带二者的 `value` 与 `disclaimer`，并保留低置信度提示。

AI 结果可用时，输出：

```text
用户：{userName}
累计获赞：{likes}；评论回复：{replies}
账号标记：{“已被站内标记为恶意账号”或“无”}

AI 评论行为总结（生成于 {generatedAt}）：
{result.profile.summary}

犀利点评：
{result.roast}

表达风格：{expressionStyle，用“、”连接；为空则“暂无足够证据”}
观点倾向：{opinionTendency，用“、”连接；为空则“暂无足够证据”}
互动模式：{interactionPattern，用“、”连接；为空则“暂无足够证据”}

依据与限制：
- {evidence[0].claim}：{evidence[0].reason}
- …
- 局限：{limitations，用“；”连接}
```

`generatedAt` 是 Unix 秒级时间戳，应按 Bot 面向用户的时区格式化。`evidenceIds`（如 `c1`）是后端脱敏样本编号，不能映射为公开评论 ID；面向普通用户时可以省略编号，只保留 `claim` 和 `reason`。

AI 结果不可用时，仍输出已取得的基础资料，并写明：

```text
AI 评论行为总结暂不可用：{none/failed/请求错误的简短原因}。
```

不要基于评论接口自行概括为“AI 总结”，除非明确标记为“Bot 根据当前展示评论整理”，并说明它不等同于后端 AI 画像。

## 可选：展示评论

```http
GET <API_BASE_URL>/user_detail/comments?userId=3566815&page=1&limit=10
Authorization: Bearer <token>
```

评论按 `approveCount`、`createTime` 降序返回。可展示 `content`、`approveCount`、`replyTotal`、`createTime`、`choice`；`choice: 1` 表示“支持用户（适合展示）”，`choice: 2` 表示“支持商家（不适合展示）”。对含个人信息、攻击性内容或匿名内容，遵循 Bot 自身的内容与隐私策略。
