# Product Variant Catalog（商品变体管理工具）

仅依赖 Go 标准库的命令行服务，帮助运营维护商品的规格维度与 SKU，并按顾客已选条件返回可售组合。

## 启动

```
go run ./cmd/catalog --state ./state
```

- `--state` 为状态目录（必填，不存在会自动创建）。
- 程序从标准输入**逐行**读取 JSON 命令，每行一条，向标准输出逐行返回 JSON。
- 每条写命令成功后立即落盘（`products.json`、`skus.json`，临时文件 + 重命名的原子写），退出重开自动恢复。

## 通用响应

成功：`{"ok":true}` 或 `{"ok":true,"result":...}`
失败：`{"ok":false,"error":"...错误信息（包含商品或 SKU 编号）..."}`

某一行报错不影响后续行的处理。

## 命令格式

### 1. 创建商品 `create_product`

商品编号全局唯一非空；至少一个维度；维度名非空且唯一；各维度值非空且不重复。创建后规格不可变更。

```json
{"cmd":"create_product","id":"shirt","dimensions":[
  {"name":"color","values":["red","blue"]},
  {"name":"size","values":["S","L"]}
]}
```

### 2. 批量添加 SKU `add_skus`

SKU 编号全局唯一；每个 SKU 必须**恰好**为所属商品的每个维度选择一个合法值；`price` 为非负整数分，`stock` 为非负整数；新建 SKU 默认为上架。

同一商品的规格组合不能重复，比较时维度顺序无关（`{"color":"red","size":"S"}` 与 `{"size":"S","color":"red"}` 视为同一组合）。**下架 SKU 仍然占用编号和组合。**

批次中出现任何重复编号、重复/冲突组合或非法值，**整批拒绝**，不会先写入部分 SKU。

```json
{"cmd":"add_skus","product":"shirt","skus":[
  {"id":"rs","attrs":{"color":"red","size":"S"},"price":1000,"stock":5},
  {"id":"bl","attrs":{"size":"L","color":"blue"},"price":1200,"stock":3}
]}
```

### 3. 上/下架 `set_listed`

```json
{"cmd":"set_listed","id":"rs","listed":false}
```

### 4. 批量更新价格与库存 `update_skus`

`price`、`stock` 可只给其一；任一项引用未知 SKU 或为负数时整批不改变状态。

```json
{"cmd":"update_skus","updates":[
  {"sku":"rs","price":900},
  {"sku":"bl","stock":0}
]}
```

### 5. 筛选查询 `query`

返回满足全部已选条件、**已上架且库存大于零**的 SKU，按编号升序排列。空筛选表示查看全部可售组合；合法但无匹配时返回空列表；未知维度或未知选项报错。

```json
{"cmd":"query","product":"shirt","filter":{"color":"red"}}
```

响应：

```json
{"ok":true,"result":{
  "skus":[{"id":"rs","attrs":{"color":"red","size":"S"},"price":1000,"stock":5}],
  "options":{"color":["red","blue"],"size":["S"]}
}}
```

## 可售选项（options）规则

`options` 给出每个维度当前**还能选到的值**，用于前端做规格置灰：

- 计算某个维度的选项时，**保留其他维度的已选筛选**，但**忽略该维度自身的已选值**（即回答“我改选这个维度时还有哪些值可走通”）。
- 只统计实际可售 SKU（已上架、库存 > 0）支持的值，而不是返回规格全集。
- 即使当前筛选组合无匹配（例如所选组合缺货），`options` 仍会显示如何改选才能回到有货组合。

例：红/L 缺货但蓝/L 有货，筛选 `{"color":"red","size":"L"}` 时 `skus` 为空，而 `options.color` 为 `["blue"]`、`options.size` 为 `["S"]`。

## 测试

```
go test ./...
```

覆盖内容：

- 商品/维度/选项非法校验与重复检测；
- SKU 批量添加的组合冲突、顺序无关比较与整批原子性；
- 下架 SKU 仍占用组合与编号；
- 批量改价/库存的整批原子性；
- 查询中的下架、缺货（库存为 0）、跨维度筛选及可售选项计算；
- 未知维度/选项报错、错误行继续处理；
- 落盘后重启的状态恢复（命令行端到端测试）。
