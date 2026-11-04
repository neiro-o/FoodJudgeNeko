# MongoDB → Elasticsearch 字段转换说明

本文档描述 `converter.py` 中，MongoDB `problems` 集合的单条 BSON 文档如何转换为 Elasticsearch 文档（`_source`）。不涉及连接、读取、批量写入等流程。

转换入口：`convert_bson_to_es(bson_doc)`，根据 `detail.taskInfo.processType` 自动路由到对应类型的转换函数。

---

## 1. 类型路由（processType → problem_type）

| MongoDB `detail.taskInfo.processType` | ES `problem_type` | 说明 |
|--------------------------------------|-------------------|------|
| `WAIMAI_COMMENT` | 1 | 外卖评价 |
| `DAODIAN_MEAL` | 2 | 堂食评价 |
| `STAR_COMMENT` | 2 | 堂食/服务评价 |
| `MINSU_COMMENT` | 2 | 堂食/服务评价 |
| `DAODIAN_HOTEL` | 2 | 堂食/服务评价 |
| `DAODIAN_TICKET` | 2 | 堂食/服务评价 |
| `DAODIAN_COMPLEX` | 2 | 堂食/服务评价 |
| `WAIMAI_QUIK_REFUND` | 3 | 外卖退款 |
| `DAOZONG_JIAOYI` | 4 | 堂食退款 |
| `IPR` | 5 | 知识产权（Logo 对比） |
| 其他未知值 | 2 | 回退到 Type 2 转换器 |

---

## 2. ES 文档顶层字段总览

下表列出 ES 文档中所有顶层字段。各 Type 的差异见第 3 节。

| ES 字段 | 类型 | 说明 |
|---------|------|------|
| `mongo_id` | string | MongoDB `_id` 转为字符串 |
| `stars` | number | 评分（0–5，步长 0.1） |
| `user_review` | string | 用户主文案 |
| `review_pics` | string[] | 评价/对比图片 URL 列表 |
| `timestamp` | int | Unix 时间戳（秒） |
| `others` | string | 附加摘要信息 |
| `problem_type` | int | 问题类型（1–5） |
| `answer` | int | 投票结论：1 或 2 |
| `ratio_1` | int | 投票比例 1 |
| `ratio_2` | int | 投票比例 2 |
| `uploader` | any | 上传者 |
| `taskId` | any | 任务 ID |
| `userId` | any | 用户 ID |
| `created_at` | int | 入库时间戳（秒） |
| `replies` | object[] | 回复列表 |
| `appeals` | object[] | 举证/申诉列表 |
| `order_info` | object \| null | 订单原始信息（堂食类） |
| `orders` | object[] | 商品/退款商品列表 |
| `order_detail` | object \| null | 外卖订单详情 |
| `comments` | object[] | 评论区内容 |

索引时，`mongo_id` 同时用作 ES 文档 `_id`（字符串形式）。

---

## 3. 各 Type 顶层字段转换规则

路径前缀省略为 `detail.taskInfo`，除非另有说明。

### 3.1 公共字段（所有 Type）

| ES 字段 | MongoDB 来源 | 转换规则 |
|---------|-------------|----------|
| `mongo_id` | `_id` | `str(_id)`，缺失则为 `null` |
| `problem_type` | — | 固定值，见第 1 节 |
| `uploader` | `uploader` | 原样 |
| `taskId` | `taskId` | 原样 |
| `userId` | `userId` | 原样 |
| `created_at` | `upload_timestamp` | 转为 `int`；缺失或非法则为 `0` |

---

### 3.2 Type 1 — 外卖评价（`WAIMAI_COMMENT`）

| ES 字段 | MongoDB 来源 | 转换规则 |
|---------|-------------|----------|
| `stars` | `voteContent.review.star` | `int(star) / 10`；缺失或非法则为 `0` |
| `user_review` | `voteContent.review.reviewContent` | 原样，默认 `""` |
| `review_pics` | `voteContent.review.reviewPicList` | 原样数组；非数组则为 `[]` |
| `timestamp` | `voteContent.review.addTime` | 毫秒 → 秒：`int(addTime) / 1000` |
| `others` | `voteContent.merchant.poiCategoryList` + `voteContent.merchant.city` | 品类用 `,` 拼接，再拼 `" in {city}"`；无 city 则只输出品类 |
| `answer` | `newOpposeRatio` vs `newSupportRatio` | 转为 int 后：`oppose > support` → `1`，否则 `2` |
| `ratio_1` | `newOpposeRatio` | 转为 `int`，非法则为 `0` |
| `ratio_2` | `newSupportRatio` | 转为 `int`，非法则为 `0` |
| `replies` | `voteContent.replyList` | 见 [4.1 replies](#41-replies)（Type 1 角色规则） |
| `appeals` | `evidenceList` | 见 [4.2 appeals](#42-appeals)（Type 1/2/3 角色规则） |
| `order_info` | — | 固定 `null` |
| `orders` | `voteContent.order.takeawayOrder.productInfoDtoList` | 见 [4.3 orders（Type 1）](#43-orders) |
| `order_detail` | `voteContent.order.takeawayOrder` | 见 [4.4 order_detail](#44-order_detail) |
| `comments` | `comment[].pageContent[]` | 见 [4.5 comments（Type 1/2/3）](#45-comments) |

---

### 3.3 Type 2 — 堂食评价

与 Type 1 相同的部分：`stars`、`user_review`、`review_pics`、`timestamp`、`others`、`answer`、`ratio_1`、`ratio_2`、`appeals`、`comments`（路径与 Type 1 一致）。

| ES 字段 | MongoDB 来源 | 转换规则 |
|---------|-------------|----------|
| `replies` | `voteContent.replyList` | 见 [4.1 replies](#41-replies)（**Type 2 角色规则**） |
| `order_info` | `voteContent.order` + `voteContent.merchant` | 见 [4.6 order_info（Type 2）](#46-order_info) |
| `orders` | — | 固定 `[]` |
| `order_detail` | — | 固定 `null` |

---

### 3.4 Type 3 — 外卖退款（`WAIMAI_QUIK_REFUND`）

| ES 字段 | MongoDB 来源 | 转换规则 |
|---------|-------------|----------|
| `stars` | — | 固定 `0` |
| `user_review` | `evidenceList`（首条举证） | 见下方说明 |
| `review_pics` | — | 固定 `[]` |
| `timestamp` | `evidenceList`（首条举证） | 见下方说明 |
| `others` | `voteContent.refundInfo.takeawayRefundInfoDto.refundSum` | 有值：`"Refund: ¥{refundSum}"`；否则 `""` |
| `answer` | `newOpposeRatio` vs `newSupportRatio` | 同 Type 1 |
| `ratio_1` | `newOpposeRatio` | 同 Type 1 |
| `ratio_2` | `newSupportRatio` | 同 Type 1 |
| `replies` | — | 固定 `[]` |
| `appeals` | `evidenceList` | 见 [4.2 appeals](#42-appeals)；**按 timestamp 升序排序** |
| `user_review` / `timestamp` | 排序后的 `appeals[0]` | `user_review` = 首条 `content`；`timestamp` = 首条 `timestamp`；无举证则分别为 `""` / `0` |
| `order_info` | — | 固定 `null` |
| `orders` | `voteContent.refundInfo.takeawayRefundInfoDto.refundProductionList` | 见 [4.3 orders（Type 3）](#43-orders) |
| `order_detail` | `voteContent.order.takeawayOrder` | 同 Type 1，见 [4.4 order_detail](#44-order_detail) |
| `comments` | `comment[].pageContent[]` | 同 Type 1，见 [4.5 comments](#45-comments) |

---

### 3.5 Type 4 — 堂食退款（`DAOZONG_JIAOYI`）

| ES 字段 | MongoDB 来源 | 转换规则 |
|---------|-------------|----------|
| `stars` | — | 固定 `0` |
| `user_review` | 排序后 `appeals[0].content` | 同 Type 3 逻辑 |
| `review_pics` | — | 固定 `[]` |
| `timestamp` | 排序后 `appeals[0].timestamp` | 同 Type 3 逻辑 |
| `others` | `voteContent.order.snapshotOrder.orderPrice` | 有值：`"Refund: ¥{orderPrice}"`；否则 `""` |
| `answer` | `newSupportRatio` vs `newOpposeRatio` | **`support > oppose` → `1`，否则 `2`（与 Type 1/3 相反）** |
| `ratio_1` | `newSupportRatio` | 转为 `int`（**与 Type 1 对调**） |
| `ratio_2` | `newOpposeRatio` | 转为 `int`（**与 Type 1 对调**） |
| `replies` | — | 固定 `[]` |
| `appeals` | `evidenceList` | 见 [4.2 appeals（Type 4 角色规则）](#42-appeals) |
| `order_info` | `voteContent.order.snapshotOrder` | 见 [4.6 order_info（Type 4）](#46-order_info) |
| `orders` | — | 固定 `[]` |
| `order_detail` | — | 固定 `null` |
| `comments` | `comment[].pageContent[]` | 见 [4.5 comments（Type 4/5）](#45-comments) |

---

### 3.6 Type 5 — 知识产权（`IPR`）

| ES 字段 | MongoDB 来源 | 转换规则 |
|---------|-------------|----------|
| `stars` | — | 固定 `0` |
| `user_review` | `voteContent.logoInfo.prosecutorLogoUrl` + `defendantLogoUrl` | 分别取 URL 文件名，拼接为 `"{filename1} vs {filename2}"` |
| `review_pics` | 同上两个 URL | 非空 URL 依次放入数组 |
| `timestamp` | `voteEndTime` | `int(voteEndTime) / 1000` |
| `others` | — | 固定 `""` |
| `answer` | `newSupportRatio` vs `newOpposeRatio` | 同 Type 4：`support > oppose` → `1`，否则 `2` |
| `ratio_1` | `newSupportRatio` | 同 Type 4 |
| `ratio_2` | `newOpposeRatio` | 同 Type 4 |
| `replies` | — | 固定 `[]` |
| `appeals` | — | 固定 `[]` |
| `order_info` | — | 固定 `null` |
| `orders` | — | 固定 `[]` |
| `order_detail` | — | 固定 `null` |
| `comments` | `comment[].pageContent[]` | 同 Type 4，见 [4.5 comments（Type 4/5）](#45-comments) |

**URL 文件名提取**（`extract_filename_from_url`）：
1. 去掉 `?` 后 query 参数
2. 取最后一个 `/` 后的片段作为文件名

---

## 4. 嵌套结构转换规则

### 4.1 replies

**来源**：`detail.taskInfo.voteContent.replyList`（数组，跳过非 dict 元素）

每条 reply 转换为：

| ES 子字段 | MongoDB 来源 | 转换规则 |
|-----------|-------------|----------|
| `role` | `replyType`（+ Type 2 额外用 `replyName`） | 见下方角色映射 |
| `timestamp` | `addTime` | 毫秒 → 秒 |
| `content` | `replyContent` | 原样，默认 `""` |

转换完成后，**按 `timestamp` 升序排序**。

#### 角色映射 — Type 1

| `replyType` | ES `role` |
|-------------|-----------|
| `MERCHANT` | `merchant` |
| `CUSTOMER` | `user` |
| 空 / 其他 | `others_{index}`，`index` 从 0 递增 |

#### 角色映射 — Type 2

| `replyType` | ES `role` |
|-------------|-----------|
| `MERCHANT` | `merchant` |
| `CUSTOMER` | `user` |
| 其他 | 从 `replyName` 中提取第一个数字 `N` → `others_{N}`；无数字则 `others_0` |

---

### 4.2 appeals

**来源**：`detail.taskInfo.evidenceList`（数组，跳过非 dict 元素）

每条 evidence 转换为：

| ES 子字段 | MongoDB 来源 | 转换规则 |
|-----------|-------------|----------|
| `role` | `belongType` | 见下方角色映射 |
| `timestamp` | `createTime` | 毫秒 → 秒 |
| `content` | `txtContent` | 原样，默认 `""` |
| `pics` | `pictureInfoDtoList` + `videoInfoDtoList` | 见 [4.7 图片/视频 URL 提取](#47-图片视频-url-提取) |

Type 3 / 4 转换后 **按 `timestamp` 升序排序**。

#### 角色映射 — Type 1 / 2 / 3

| `belongType` | ES `role` |
|--------------|-----------|
| `PROSECUTOR_EVIDENCE` | `merchant` |
| 其他 | `user` |

#### 角色映射 — Type 4（与上表相反）

| `belongType` | ES `role` |
|--------------|-----------|
| `PROSECUTOR_EVIDENCE` | `user` |
| 其他 | `merchant` |

---

### 4.3 orders

#### Type 1 — 来源：`voteContent.order.takeawayOrder.productInfoDtoList`

#### Type 3 — 来源：`voteContent.refundInfo.takeawayRefundInfoDto.refundProductionList`

每条 product 转换为：

| ES 子字段 | MongoDB 来源 | 转换规则 |
|-----------|-------------|----------|
| `name` | `name` | 原样，默认 `""` |
| `count` | `count` | 转为 `int`，非法则为 `0` |
| `desc` | `description` | 原样，默认 `""` |
| `selection` | `attrValueList` | 原样数组；非数组则为 `[]` |
| `pic` | `picUrl` | 原样，默认 `""` |
| `others` | `spec`、`minOrderCount`（Type 3 额外含 `price`） | 见下方 |

**`others` 拼接规则**：

- **Type 1**：`spec`；若有 `minOrderCount` 则追加 `"Minimum per order: {minOrderCount}"`；用 `", "` 连接
- **Type 3**：`spec`；若有 `price` 则追加 `"Price: {price}"`；若有 `minOrderCount` 则追加 `"Minimum per order: {minOrderCount}"`；用 `", "` 连接

---

### 4.4 order_detail

**来源**：`voteContent.order.takeawayOrder`（Type 1、Type 3）

若 `takeawayOrder` 不是 dict，则 ES `order_detail` 为 `null`。

| ES 子字段 | MongoDB 来源 | 转换规则 |
|-----------|-------------|----------|
| `order_started` | `orderTime` | 毫秒 → 秒 |
| `order_finished` | `finishTime` | 毫秒 → 秒 |
| `deliver_time` | `deliveryTimeTakeMs` | 毫秒 → 秒；缺失或非法则为 `-1` |
| `total_time` | `orderTimeTakeMs` | 毫秒 → 秒 |
| `deliver_by` | `deliveryTypeCn` | 见 [4.8 配送方式映射](#48-配送方式映射) |
| `note` | `remarkContent` | 原样，默认 `""` |
| `utensils` | `needTableware` + `tablewareCount` | `needTableware == true` → `int(tablewareCount)`（`-1` 保留）；否则 `0` |
| `invoice` | `needInvoice` | 原样布尔值 |

---

### 4.5 comments

**来源**：顶层 `comment` 数组 → 每个元素的 `pageContent` 数组（两层嵌套，跳过非 dict 元素）

每条 comment 转换为：

| ES 子字段 | MongoDB 来源 | 转换规则 |
|-----------|-------------|----------|
| `userid` | `userId` | 优先转为 `int`；失败则保留原值；缺失为 `0` |
| `name` | `userName` | 原样，默认 `""` |
| `content` | `content` | 原样，默认 `""` |
| `timestamp` | `createTime` | 毫秒 → 秒 |
| `likes` | `approveCount` | 转为 `int`；缺失或非法则为 `-1` |
| `choice` | `voteOperate` | 见下方 |

#### choice 映射

**Type 1 / 2 / 3**：

| `voteOperate` | ES `choice` |
|---------------|-------------|
| `DOWN` | `1` |
| `UP` | `2` |
| 其他 / 缺失 | `0` |

**Type 4 / 5**（与上表相反）：

| `voteOperate` | ES `choice` |
|---------------|-------------|
| `DOWN` | `2` |
| `UP` | `1` |
| 其他 / 缺失 | `0` |

---

### 4.6 order_info

#### Type 2

```json
{
  "raw": "<voteContent.order 原样>",
  "shop_info": {
    "name": "<voteContent.merchant.poiName，默认 \"\">",
    "avg_price": "<int(voteContent.merchant.poiAvgPrice)，非法则为 0>"
  },
  "processed": null
}
```

#### Type 4

```json
{
  "raw": "<voteContent.order.snapshotOrder 原样>",
  "processed": null
}
```

Type 1 / 3 / 5 的 `order_info` 均为 `null`。

---

### 4.7 图片/视频 URL 提取

用于 `appeals[].pics`，来源为 `pictureInfoDtoList` 与 `videoInfoDtoList`。

**图片对象**（dict）：按顺序尝试 `url` → `picUrl` → `coverUrl` → `imageUrl`，取第一个非空值。

**视频对象**（dict）：按顺序尝试 `m3u8Url` → `videoUrl` → `coverUrl`，取第一个非空值。

**字符串元素**：直接作为 URL 追加。

最终合并为一个 URL 字符串数组。

---

### 4.8 配送方式映射

用于 `order_detail.deliver_by`，来源为 `takeawayOrder.deliveryTypeCn`：

| MongoDB `deliveryTypeCn` | ES `deliver_by` |
|--------------------------|-----------------|
| `美团配送` | `meituan` |
| `商家配送` | `merchant` |
| `顾客自取` | `user` |
| `用户自取` | `user` |
| 空 / 其他 | `others` |

---

## 5. 各 Type 字段差异速查

| 字段 | Type 1 | Type 2 | Type 3 | Type 4 | Type 5 |
|------|--------|--------|--------|--------|--------|
| `stars` | review.star÷10 | 同左 | `0` | `0` | `0` |
| `user_review` | reviewContent | 同左 | appeals[0].content | 同左 | logo 文件名对比 |
| `review_pics` | reviewPicList | 同左 | `[]` | `[]` | 两个 logo URL |
| `timestamp` | review.addTime | 同左 | appeals[0].timestamp | 同左 | voteEndTime |
| `others` | 品类+city | 同左 | Refund 金额 | Refund 金额 | `""` |
| `answer` 逻辑 | oppose>support→1 | 同左 | 同左 | **support>oppose→1** | 同 Type 4 |
| `ratio_1` | oppose | 同左 | 同左 | **support** | **support** |
| `ratio_2` | support | 同左 | 同左 | **oppose** | **oppose** |
| `replies` | 有 | 有（不同 role 规则） | `[]` | `[]` | `[]` |
| `appeals` | 有 | 有 | 有（排序） | 有（排序，role 反转） | `[]` |
| `order_info` | `null` | 有 shop_info | `null` | 有 snapshotOrder | `null` |
| `orders` | 商品列表 | `[]` | 退款商品 | `[]` | `[]` |
| `order_detail` | 有 | `null` | 有 | `null` | `null` |
| `comments.choice` | DOWN→1 | 同左 | 同左 | **DOWN→2** | **DOWN→2** |

---

## 6. 默认值与异常处理

- 所有嵌套路径读取均通过 `safe_get()`，路径不存在时返回指定默认值，不抛 `KeyError`。
- 数值转换（`int`、除法）失败时，使用该字段文档中标注的 fallback 值（通常为 `0`、`-1` 或 `""`）。
- 数组来源字段若非 list，则输出空数组 `[]`。
- 非 dict 的数组元素在遍历时会跳过。

---

## 7. 参考实现

转换逻辑源码：`converter.py`

- `convert_bson_to_es()` — 自动路由
- `convert_bson_to_es_type1()` ~ `convert_bson_to_es_type5()` — 各类型转换
- 辅助函数：`safe_get`、`convert_reply_role`、`convert_reply_role_type2`、`convert_delivery_type`、`extract_pic_urls`、`extract_filename_from_url`
