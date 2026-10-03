# 商品变体目录（Product Variant Catalog）

仅使用 Go 标准库实现的商品规格 / SKU 管理工具。进程通过标准输入逐行
接收 JSON 命令，逐行返回 JSON 响应；状态持久化在 `--state` 指定目录中，
退出重开后数据保留。

## 运行

```bash
go run ./cmd/catalog --state ./state
```

- 每行输入必须是一个完整 JSON 对象（不支持跨行 JSON）。
- 空行忽略；非法 JSON 行返回错误响应，后续行继续处理。
- 每条成功的写命令都会立即把状态原子写入 `<state目录>/catalog-state.json`
  （临时文件 + 重命名），因此进程中断不会留下半个状态文件。

## 响应格式

成功：

```json
{"ok":true,"result":{ ... }}
```

失败（错误信息会定位到商品或 SKU）：

```json
{"ok":false,"error":"sku \"s1\": price must be a non-negative integer"}
```

单行出错不影响后续命令的处理。

## 命令

### 1. 创建商品 `create_product`

商品编号唯一非空；至少一个规格维度；维度名非空且在商品内唯一；每个
维度至少一个值，值非空且不重复。创建后规格维度不再变更。

```json
{"cmd":"create_product","data":{"id":"shirt","dimensions":[
  {"name":"color","values":["red","blue"]},
  {"name":"size","values":["s","m","l"]}
]}}
```

结果：`{"id":"shirt"}`。

### 2. 批量添加 SKU `add_skus`

SKU 编号全局唯一、非空。每个 SKU 必须恰好为所属商品的**每个**维度选
一个合法值（维度书写顺序任意）；价格、库存为非负整数；新 SKU 初始为
上架。

```json
{"cmd":"add_skus","data":{"product":"shirt","skus":[
  {"id":"s1","values":{"color":"red","size":"s"},"price":1000,"stock":2},
  {"id":"s2","values":{"size":"m","color":"blue"},"price":1500,"stock":0}
]}}
```

结果：`{"added":["s1","s2"]}`（按编号排序）。

同一商品的规格组合不可重复，比较不受维度键顺序影响；**已下架的 SKU
仍占用其编号与组合**。

整批是原子的：编号重复（批内或全局）、组合冲突（批内或与既有 SKU）、
任一价格/库存非法、维度缺失/多余/值非法时，整批拒绝，不会先写入部分
SKU。

### 3. 上 / 下架 `set_active`

```json
{"cmd":"set_active","data":{"sku":"s1","active":false}}
```

下架不删除 SKU，其编号与组合继续被占用；下架后查询中不可售。

### 4. 批量更新价格 / 库存 `update_skus`

每个更新项只需给出要改的字段；未给出的字段保持不变。价格、库存必须
为非负整数；任一 SKU 不存在、批内重复或数值非法时整批不生效。

```json
{"cmd":"update_skus","data":{"updates":[
  {"id":"s1","price":1200},
  {"id":"s2","stock":5}
]}}
```

结果：`{"updated":["s1","s2"]}`。

### 5. 筛选查询 `query`

`select` 给出部分维度的已选值。返回同时满足全部条件、**已上架且库存
大于零**的 SKU（按编号排序），以及每个维度的可售选项。

```json
{"cmd":"query","data":{"product":"shirt","select":{"color":"red"}}}
```

```json
{"ok":true,"result":{
  "skus":[{"id":"s1","values":{"color":"red","size":"s"},"price":1000,"stock":2}],
  "available":{"color":["red","blue"],"size":["s"]}
}}}
```

- `select` 为空（`{}` 或缺省）表示查看全部可售组合。
- 合法但无匹配的筛选正常返回空 `skus` 列表，不是错误。
- 未知维度或维度的未知选项返回错误。

#### 可售选项（`available`）规则

对**每个**维度单独计算：

1. 保留其他所有维度的已选筛选；
2. 忽略该维度自身的已选值；
3. 只统计已上架、库存大于零的 SKU；
4. 列出这些 SKU 在该维度上实际出现的选项（按规格定义顺序）。

因此选项不是规格全集，而是“切到该值后仍有可售 SKU”的值。例如已选
`color=red` 时：

- `available.size` 只保留 `color=red` 的可售 SKU 支持的尺码；
- `available.color` 会忽略 `color=red` 自身（等价于颜色不做筛选），
  列出全部可售颜色，供顾客切换。

## 测试

```bash
go test ./...
```

覆盖内容：

- 商品 / 维度 / SKU 的各类非法输入与原子批次（重复编号、冲突组合）；
- 下架 SKU 仍占用组合、下架后查询不可售；
- 库存为零的 SKU 从结果与可售选项中排除；
- 跨维度筛选、维度自身已选值在选项计算中被忽略；
- 未知维度 / 选项报错、无匹配返回空列表；
- 写盘后重启进程状态保留；
- 非法 JSON 行只产生错误响应，不中断后续行。
