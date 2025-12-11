# ES字段解释说明

## 顶层字段

| 字段名         | 类型              | 说明                                                                 |
| -------------- | ----------------- | -------------------------------------------------------------------- |
| `id`           | string / number   | 评价记录的唯一标识符。                                               |
| `mongo_id`     | string            | MongoDB中的唯一标识符。                                               |
| `stars`        | number            | 用户评分。1-5分，1分为最差，5分为最好。                                                         |
| `user_review`  | string            | 用户的文字评价内容。可能为空（退款题）                               |
| `review_pics`  | array[string]      | 用户上传的评论图片 URL 列表。                                        |
| `timestamp`    | number            | 用户评论时间戳（单位：秒）。                                         |
| `others`       | string            | 其他补充说明，例如"消费后评价"等。                                   |
| `problem_type`  | number            | 问题类型：1-外卖，2-堂食，3-外卖退款，4-堂食退款，5-其他             |
| `answer`       | number            | 评审结果答案。`1`为支持用户(适合展示/适合退款等)，`2`为支持商家(不适合展示/不适合退款等) |
| `ratio_1`      | number            | 评审比例1，即选1的占比(0~100)。                                      |
| `ratio_2`      | number            | 评审比例2，即选2的占比(0~100)。                                      |
| `uploader`     | string            | 上传此题用户的ID                                                     |
| `taskId`       | string            | 关联任务ID                                                           |
| `userId`       | string            | 原链接的userID参数                                                           |
| `created_at`   | number            | 创建时间(时间戳,单位:秒)                                             |

---

## 回复与申诉

### `replies` (list)

商家的回复一般是一个时间线，因为可能有用户追评，也可能有路人。

| 子字段     | 类型   | 说明                                                                 |
| ---------- | ------ | -------------------------------------------------------------------- |
| `role`     | string | 发送方身份：`merchant`（商家）或 `user`（顾客） 或 `others_X`（路人X，X为数字）。 |
| `timestamp` | number | 回复时间戳（单位：秒）。                                             |
| `content`  | string | 回复内容。                                                           |

### `appeals` (list)

根据退款题的经验，用户和商家都会有申诉理由。

| 子字段     | 类型            | 说明                                 |
| ---------- | --------------- | ------------------------------------ |
| `role`     | string          | 同`replies`中的`role`。               |
| `timestamp` | number          | 申诉提交时间（单位：秒）。             |
| `content`  | string          | 申诉文字说明，例如证据描述。           |
| `pics`     | array[string]   | 申诉佐证图片链接。                     |

---

## 订单信息

### `order_info` (a variable dict)

仅在非外卖类问题中有效（如团购信息说明），若为空则为外卖类。

### `orders` (list)

| 子字段     | 类型            | 说明                                                               |
| ---------- | --------------- | ------------------------------------------------------------------ |
| `name`     | string          | 商品名称。                                                         |
| `count`    | number          | 购买数量。                                                         |
| `desc`     | string          | 商品描述。                                                         |
| `selection` | array[string]   | 用户选择的规格或口味，例如 `["热", "不额外加糖"]`。                 |
| `pic`      | string          | 商品展示图 URL。                                                   |
| `others`   | string          | 其他附加信息或备注。                                               |

---

## 订单详细信息（外卖场景专用）

### `order_detail` (dict)

| 子字段           | 类型    | 说明                                                                 |
| ---------------- | ------- | -------------------------------------------------------------------- |
| `order_started`  | number  | 下单时间戳（s，下同）。                                               |
| `order_finished` | number  | 完成时间戳（自取时为0）。                                             |
| `deliver_time`   | number  | 配送耗时（秒），自取或商家配送时为-1（不可用）。                       |
| `total_time`     | number  | 从下单到完成的总时长（秒），包含预订单的时间。                         |
| `deliver_by`     | string  | 配送方：如`merchant`(商家配送)、`meituan`(美团骑手)、`user`(用户自取)。 |
| `note`           | string  | 用户备注，例如"给我200块钱再给骑手100"。                             |
| `utensils`       | number  | 餐具份数，`0`为环保单，`-1`为按需提供，其他为指定数值。               |
| `invoice`        | boolean | 是否开具发票。（目前没看到需要开发票的题）                           |

---

## 题目评价信息

### `comments` (list)

| 子字段     | 类型   | 说明                                           |
| ---------- | ------ | ---------------------------------------------- |
| `userid`   | number | 评论用户ID。                                     |
| `name`     | string | 评论用户昵称。                                   |
| `content`  | string | 评论内容。                                       |
| `timestamp` | number | 评论时间戳（单位：秒）。                         |
| `choice`   | number | 评论选择：`1`为支持用户，`2`为支持商家。         |
| `likes`    | number | 点赞数。                                       |
