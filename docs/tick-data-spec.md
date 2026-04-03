# Tick Data Spec

这份文档定义平台当前可接受的 `tick` 执行层数据格式。目标很简单：把真实成交或逐笔价格数据放进指定目录后，平台能自动发现、展示并用于回测。

## 目录约定

推荐把 tick 数据放到：

```text
dataset/archive/
```

配套环境变量：

```env
TICK_DATA_DIR=./dataset/archive
```

平台同时支持两种 tick 数据组织方式：

1. 清洗后的单文件或少量文件
2. Binance 月度 archive 分片目录

## 文件命名

平台当前通过文件名自动推断标的，命名规则建议如下：

- `BTC_tick_Clean.csv`
- `BTC_tick.csv`
- `ETH_tick_Clean.csv`
- `ETH_tick.csv`

规则说明：

- 文件名前缀会被解释成基础币种代码，例如 `BTC`、`ETH`
- 平台会自动把前缀映射成 `BTCUSDT`、`ETHUSDT`
- 如果后续要支持非 `USDT` 结算对，再扩展命名约定

对于 Binance 月度 archive，平台也支持这种目录结构：

- `BTCUSDT-trades-2020-01/BTCUSDT-trades-2020-01.csv`
- `BTCUSDT-trades-2020-02/BTCUSDT-trades-2020-02.csv`

平台会自动把这些月度目录聚合成一个 `BTCUSDT` tick 数据集。

## CSV 字段

必需列：

- `timestamp`
- `price`

可选列：

- `quantity`
- `side`

当前示例：

```csv
timestamp,price,quantity,side
2026-01-01T00:00:00Z,95432.1,0.015,buy
2026-01-01T00:00:01Z,95431.8,0.008,sell
```

## 字段说明

- `timestamp`
  - 推荐使用 RFC3339 / ISO 8601 UTC 时间，例如 `2026-01-01T00:00:00Z`
  - 当前平台至少要求该列存在且非空
- `price`
  - 成交价或逐笔价格
  - 数值型
- `quantity`
  - 可选，表示该笔的成交数量或事件数量
- `side`
  - 可选，建议使用 `buy` / `sell`

## 当前平台行为

平台会：

- 扫描 `TICK_DATA_DIR` 下符合命名规则的 CSV 文件
- 识别 Binance 月度 archive 目录并按标的聚合
- 先构建轻量 manifest，再按月文件流式读取，不会把多年 tick 全量并入内存
- 在 `GET /api/v1/backtests/options` 中返回：
  - `availability.tick`
  - `datasets.tick`
  - `supportedSymbols.tick`
  - `schema.tick`
- 在创建回测时校验请求标的是否真的存在对应 tick 文件

平台当前不会：

- 自动清洗坏数据
- 自动按毫秒或纳秒时间戳做转换
- 自动把交易所原始逐笔格式映射成统一 schema
- 把几十 GB 的月度 tick 文件一次性加载进内存

仓库中附带了一份模板文件：

- [data/tick/BTC_tick.sample.template](/Users/wuyaocheng/Downloads/bkTrader/data/tick/BTC_tick.sample.template)

如果接入的是交易所导出的原始逐笔文件，平台现在可以直接识别 Binance 月度 archive；如果是其他来源，建议先在研究侧做一层清洗，再产出符合本规范的 `*_tick_Clean.csv`。

## 建议的数据准备流程

1. 从交易所或行情供应商导出逐笔数据。
2. 统一时间格式为 UTC RFC3339。
3. 至少保留 `timestamp` 和 `price`。
4. 文件命名成 `BTC_tick_Clean.csv`，或保留 Binance archive 原始目录结构。
5. 放入 `dataset/archive/`。
6. 启动平台后通过 `GET /api/v1/backtests/options` 或前端回测面板确认已被发现。
