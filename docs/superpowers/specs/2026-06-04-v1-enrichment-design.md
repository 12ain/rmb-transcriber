# RMB 大写转换 · v1 功能扩展设计

- **日期**：2026-06-04
- **范围**：在现有「数字 → 中文大写」单端点工具基础上，新增反向转换 + 双向校验、批量转换、API 文档页（含 OpenAPI）。
- **目标用户**：兼顾个人、财务实务人员、开发者集成三类，本期聚焦覆盖面最广的核心能力。

## 1. 目标与非目标

**v1 目标**

1. 提供反向能力：中文大写 → 阿拉伯数字；并在此之上提供「双向一致性校验」用于合同/票据审核。
2. 支持批量转换（粘贴多行），最多 200 条/请求。
3. 提供独立的 API 文档页，含 curl/Python/Node/Go 代码示例与 Try It 在线试调；同时暴露 OpenAPI 3.1 JSON，并在独立路由内嵌 Swagger UI。
4. 前端 UI 用同页 Tab 切换组织 4 种模式（转写 / 还原 / 校验 / 批量），保留现有视觉语言（Bricolage Grotesque + JetBrains Mono + Noto Serif SC，深色/浅色双主题）。

**v1 非目标**

- 多币种大写（USD/EUR/HKD 等）—— 留 v2。
- 历史记录、收藏、PNG/PDF 导出 —— 留 v2。
- API Key、限流、用量统计 —— 留 v2。
- 「圆/元」「整/正」「负数」作为前端可见开关 —— v1 输出端默认央行口径（「圆 + 整」），仅反向接口宽容输入；不暴露切换 UI，保持界面精简。
- 任何前端测试框架引入。

## 2. 架构总览

- **逻辑放后端**：所有转换、解析、校验均在 Go 端实现，前端只渲染。避免双实现走样，确保 API 服务的「真实可信源」。
- **多端点 REST**：四个独立端点，各司其职。
- **OpenAPI 手写**：项目体量小，不引入 swaggo 等代码生成工具，避免构建复杂度。
- **新增依赖**：零。Go 仍只用 `gin` + `regexp`；前端仍只用 CDN 上的 Vue 3、axios、Google Fonts；Swagger UI 也走 CDN（swagger-ui-dist）。

## 3. API 设计

### 3.1 端点清单

| 端点 | 用途 | 请求 | 成功响应 |
|---|---|---|---|
| `POST /api/convert` | 正向（保留兼容） | `{"amount":"1234.56"}` | `{"chinese":"壹仟贰佰叁拾肆圆伍角陆分"}` |
| `POST /api/convert/reverse` | 反向 | `{"chinese":"壹仟贰佰叁拾肆圆伍角陆分"}` | `{"amount":"1234.56"}` |
| `POST /api/convert/verify` | 双向校验 | `{"amount":"1234.56","chinese":"…"}` | `{"match":true,"expected":"…"}` 或 `{"match":false,"expected":"…","diffAt":5}` |
| `POST /api/convert/batch` | 批量正向 | `{"amounts":["1.00","2.34",…]}`（≤ 200 条） | `{"results":[{"amount":"1.00","chinese":"…"} 或 {"amount":"…","error":"invalid_format"}, …]}` |

### 3.2 兼容性

`POST /api/convert` 的请求/响应字段名与含义**不变**，确保历史前端继续工作。

### 3.3 错误码

| HTTP | code | 含义 |
|---|---|---|
| 400 | `invalid_format` | 数字格式不符（非 `^-?\d+(\.\d{1,2})?$`） |
| 400 | `out_of_range` | 数字越界（绝对值 > 999,999,999,999.99） |
| 400 | `unparsable_chinese` | 中文大写无法解析；响应附 `at` 字段指示偏移 |
| 413 | `batch_too_large` | 批量超过 200 条 |

错误响应统一形态：`{"error":"<code>","message":"<人类可读说明>","at":<可选数字>}`。

### 3.4 负数

所有端点允许 `amount` 以 `-` 开头；对应大写以「负」开头：`-1234.56` ↔ `负壹仟贰佰叁拾肆圆伍角陆分`。这是反向解析必须支持的形态，但前端 UI 不暴露专门的负数开关，用户直接输入即可。

### 3.5 「圆/元」「整/正」宽容输入

- `reverse` 与 `verify` 接受四种组合：`圆/元` × `整/正`。
- 内部规范化为 `圆 + 整` 后再比对（`verify` 的 `expected` 字段始终是规范形态）。
- 正向接口（`convert` / `batch`）**只输出**`圆 + 整`，不开放切换。

### 3.6 批量

- 请求体上限 200 条；超出返回 `413 batch_too_large`，整批不处理。
- 单条失败不影响整批；结果数组按原顺序对齐，每项含 `amount` 与（`chinese` 或 `error`）。
- 顶部聚合统计由前端从结果数组自行计算（valid 数 / error 数），后端不冗余下发。

## 4. 后端实现策略

### 4.1 文件组织

文件布局以 **§12 项目结构优化** 为准：所有转换核心逻辑下沉到 `internal/converter/` 子包（`forward.go`、`reverse.go`、`verify.go`、`batch.go`、`errors.go` 及对应测试文件），根包 `main.go` 仅保留薄 HTTP 适配层。

### 4.2 反向解析：正则切段

使用正则把字符串切成几段，各段独立解析后加权求和：

```
^(负)?  ([零壹贰叁肆伍陆柒捌玖拾佰仟万亿]+)  (圆|元)  ([零壹贰叁肆伍陆柒捌玖]角)?  ([零壹贰叁肆伍陆柒捌玖]分)?  (整|正)?$
```

- 整数段：从右向左按「亿 → 万 → 个」三段位组拆分，每段组内按「仟/佰/拾/个」解；段间相加。
- 角、分段独立成数字。
- 失败返回 `unparsable_chinese`，附首个解析失败的偏移量（用于 `at` 字段）。

不采用完整状态机或 PEG 解析器；正则切段已足够覆盖标准会计写法。

### 4.3 校验差异定位

`Verify` 先调 `ParseChinese(chinese)` 拿 `parsedAmount`：

- `parsedAmount == amount` 且字符串规范化后逐字相等 → `match: true`
- 否则计算 `expected = ConvertToChinese(amount)`，逐字比对 `chinese` 与 `expected`，返回首个不同字位置 `diffAt`。
- 若 `ParseChinese` 直接失败 → `match: false`，`expected` 仍计算，`diffAt: 0`，`message: "<解析失败说明>"`。

### 4.4 现有 `ConvertToChinese` 的回归修复

实现新功能时顺手补一处现有 bug 风险：当前 `main.go:63` 在处理零时用 `strings.HasSuffix(result.String(), "零")` 检查，零边界（如 `0`、`0.01`）尚未明确覆盖。在 `converter.go` 重写时按用例表逐条修正，旧契约不变（同输入返回相同输出，除已知错误用例外）。

## 5. 前端 UI 设计

### 5.1 整体结构

```
[NAV: ¥ logo  ·  meta  ·  DOCS↗  ·  主题切换]
        │
[HERO: 标题 + 副标题]
        │
[TABS: ● 转写  ○ 还原  ○ 校验  ○ 批量]   ← accent 色滑动指示线
        │
[WORKSPACE: 根据 Tab 渲染对应布局]
        │
[示例 ledger（点击载入到当前 Tab）]
        │
[FOOTER]
```

### 5.2 四种工作区

| Tab | 左面板 | 右面板 |
|---|---|---|
| **转写** | 大写结果（Noto Serif SC，逐字渐入）+ 字数/源/时间戳 + 复制按钮 | 数字输入 + 转写按钮 + 数据 tile（精度/上限） |
| **还原** | 还原后的数字（巨型 JetBrains Mono）+ 复制按钮 + 时间戳 | 中文大写 textarea + 还原按钮 + 数据 tile |
| **校验** | 校验结论：`✓ MATCH`（accent 绿光晕）或 `✗ MISMATCH`（双行对比 + diff 高亮） | 数字输入 + 大写 textarea + 校验按钮（数据 tile 隐藏，腾空间） |
| **批量** | 结果列表（行对齐：`#1 ¥ x.xx → 中文大写 [复制]`，失败行红色）+ 顶部聚合 chip `38 valid · 4 errors` + 「复制全部 TSV」/「导出 CSV」 | 多行 textarea + 实时行数计数 `42 / 200` + 批量按钮 |

### 5.3 状态隔离

每个 Tab 在 Vue data 内有独立 `modeState` 子对象（`forward`、`reverse`、`verify`、`batch`），存输入与结果。Tab 切换不清空，切回原样。

### 5.4 批量 Tab 客户端预校验

textarea 每次 `input` 按 `\n` 切行（过滤空行）计数：

- 计数 ≤ 200：正常显示 `N / 200`
- 计数 > 200：计数变 warn 色 `N / 200 · 超出上限`；textarea 边框变 warn 色；批量按钮 disabled；提示「已粘贴 N 条，请删除 N-200 条后再继续」

这避免无意义的 413 往返。

### 5.5 校验 Tab 不匹配可视化

```
✗  M I S M A T C H

你 输 入 的 大 写：壹仟贰佰叁拾肆圆伍角[陆]分
系 统 计 算 结 果：壹仟贰佰叁拾肆圆伍角[柒]分
                                  ↑ diffAt = 12
```

首个不同字用 accent 色 + 下划线高亮。两行字号略小于 match 状态。

### 5.6 示例 ledger 行为调整

点击示例时按当前 Tab 灌入：

- 当前 = 转写 → 把数字灌进数字输入并自动转写
- 当前 = 还原 → 把示例的中文大写灌进 textarea 并自动还原
- 当前 = 校验 → 把数字与对应大写分别灌入并自动校验
- 当前 = 批量 → 把示例数字以新行追加进 textarea（不自动提交）

### 5.7 响应式

≤ 1080px：左右面板上下堆叠（沿用现有断点）。批量 Tab 在窄屏隐藏「导出 CSV」按钮，保留「复制全部」。

## 6. API 文档页设计

### 6.1 路由

- `GET /docs` → 服务 `static/docs.html`（手写设计的长文档页）
- `GET /docs/spec` → 服务 `static/swagger.html`（CDN 加载 swagger-ui-dist 渲染 `/openapi.json`）
- `GET /openapi.json` → 服务 `static/openapi.json`
- 主页 NAV 加「DOCS ↗」链接

### 6.2 `/docs` 页面结构

```
[NAV: 复用主页 NAV 风格 + ⌘K 搜索 + 主题切换 + 返回主页]
[侧栏: 概览 / 认证 / 限流 / 错误码 / 端点列表 / OpenAPI Reference↗]
[主区: 概览 → 错误码总表 → 4 个端点章节 → 反向解析行为说明]

每个端点章节固定模板：
  描述
  请求体（JSON 示例 + 字段说明表）
  响应体（JSON 示例）
  错误码（适用项）
  代码示例 Tab：curl / Python(requests) / Node(fetch) / Go(net/http)
  Try It：内嵌实时调试器
```

### 6.3 视觉一致性

- 复用主页 CSS 变量（`--bg / --fg / --accent / …`），主题切换状态通过同一 `localStorage.theme` 同步。
- 字体：Bricolage Grotesque + JetBrains Mono；中文大写示例用 Noto Serif SC。
- Swagger UI 因有独立视觉系统，单独放在 `/docs/spec`，不与 `/docs` 混排。

### 6.4 Try It 调试器

- 同源 fetch，无需 CORS。
- 显示 HTTP 状态、耗时（ms）、响应 JSON（折叠展示）。
- 失败时显示完整 error 响应。
- **不持久化任何调试历史**（金额可能敏感）。

### 6.5 代码示例

每个端点四份样例，按各语言惯用风格手写，不用模板替换：

| 语言 | 库 |
|---|---|
| `curl` | shell |
| `Python` | `requests` |
| `Node.js` | 原生 `fetch` |
| `Go` | `net/http` + `encoding/json` |

## 7. 测试

### 7.1 Go 后端：表驱动单元测试

每个核心函数一组用例。覆盖类别：

- **零边界**：`0`、`0.00`、`0.01`、`0.1`
- **零的内部处理**：`100`、`1000`、`10000`、`100000`、`1001`、`10001`、`10010010.01`、`100000000`、`100000000.01`
- **位级单位**：`10000`、`100000000`、`10000000000`
- **小数补零/截断**：`12.3`、`12.30`、`12.34`、`12.345`
- **上限/越界**：`999999999999.99` 通过，`1000000000000` 返回 `out_of_range`
- **负数**：`-1234.56` 正反双向
- **反向宽容**：`圆/元` × `整/正` 四种组合
- **反向省略**：`壹仟圆` → `1000`
- **反向失败**：空串、含非法字符、单位顺序错乱 → `unparsable_chinese` + `at`
- **校验**：match / mismatch（含 `diffAt`）/ 数字侧无效 / 大写侧无效
- **批量**：空数组、单条、200 条临界、201 条 → `413`、混合有效与无效（顺序与数量对齐）

### 7.2 圆/元 整/正 对称性测试

对每个金额构造两种大写串（「圆+整」「元+正」），喂给 `ParseChinese` 都应得到同一数字。

### 7.3 兼容性测试

锁定 `/api/convert` 的契约：请求字段 `amount`、响应字段 `chinese`、HTTP 200 形态完全不变。

### 7.4 前端测试

不引入前端测试框架。仅在 README 加一条手动 smoke checklist：

1. 4 个 Tab 依次切换，输入与结果不互相污染。
2. 主题切换 + 模式切换的组合无样式残留。

### 7.5 CI

无新增。`go test ./...` 即可全部通过。

## 8. 隐私与安全底线

- 不做任何金额、IP、UA 的业务日志埋点。
- Gin access log 中间件对 `/api/convert`、`/api/convert/reverse`、`/api/convert/verify`、`/api/convert/batch` 的**请求体不打印**（默认 gin.Logger 不打 body，已满足；如未来切日志库需注意保留此底线）。
- Try It 调试器仅前端发起请求，无任何代理或转发。

## 9. 部署变更

- Dockerfile **移除**单独 COPY static 目录的步骤 —— 静态资源已通过 `//go:embed static/*` 编入二进制。
- 无新增运行时依赖。
- 镜像体积变化：因 static 嵌入二进制，最终镜像略增（HTML/CSS/JSON 合计 < 80KB），但层数减少；整体复杂度下降。

## 10. 已确认的关键取舍

| 议题 | 决定 |
|---|---|
| 逻辑前后端分布 | 全部后端，前端仅渲染 |
| API 形状 | 多端点 REST，非单端点 + mode 字段 |
| OpenAPI 来源 | 手写 |
| Swagger UI | 内嵌（独立路由 `/docs/spec`），CDN 加载 |
| Try It 历史 | 不持久化 |
| UI 组织 | 同页 Tab |
| 批量上限 | 200 条 |
| 批量超限处理 | 客户端粘贴时立即提示 + 禁用提交 |
| 反向解析方式 | 正则切段 |
| 前端测试框架 | 不引入 |
| Go 包布局 | 根 `main.go` + `internal/converter/` 子包 |
| 静态资源 | `//go:embed` 编入二进制 |
| CSS 共享 | 抽 `static/shared/theme.css` 两页共用 |
| 多币种、历史、导出、API Key | 全部留 v2 |

## 11. 文件清单（实现阶段产物）

详见 **§12.5 变更范围**。简表如下：

新增：

- `internal/converter/` 整包（`forward.go`、`reverse.go`、`verify.go`、`batch.go`、`errors.go` 及对应 `*_test.go`）
- `static/docs.html`、`static/swagger.html`、`static/openapi.json`
- `static/shared/theme.css`

修改：

- `main.go`（重构为薄 HTTP 适配层 + 路由 + `go:embed` 挂载）
- `static/index.html`（Tab 切换 + 4 模式工作区 + 批量预校验 + 改用 `<link>` 引用共享 CSS）
- `Dockerfile`（移除单独 COPY static 的步骤，因已 embed）
- `README.md`（功能与端点更新）

迁移：

- 现有 `ConvertToChinese` 函数与 `digits` / `units` 常量 → `internal/converter/forward.go`

## 12. 项目结构优化

### 12.1 目录与包重组

当前 Go 逻辑全部挤在根包的 `main.go`，静态资源只有一个 `index.html`。本期一并整理：

```
rmb-uppercase-converter/
├── main.go                          # HTTP 路由与启动（薄层）
├── internal/
│   └── converter/                   # 转换核心逻辑包
│       ├── forward.go               # 数字 → 大写
│       ├── reverse.go               # 大写 → 数字（正则切段）
│       ├── verify.go                # 双向校验 + diff 定位
│       ├── batch.go                 # 批量包装（单条失败隔离）
│       ├── errors.go                # 错误码常量 + ConverterError 类型
│       ├── forward_test.go
│       ├── reverse_test.go
│       ├── verify_test.go
│       └── batch_test.go
├── static/                          # go:embed 嵌入二进制
│   ├── index.html                   # 主应用（4 个 Tab）
│   ├── docs.html                    # API 文档页
│   ├── swagger.html                 # Swagger UI 容器
│   ├── openapi.json                 # OpenAPI 3.1 规范
│   └── shared/
│       └── theme.css                # 主题 CSS 变量（深色/浅色）
├── docs/
│   ├── deployment.md
│   └── superpowers/specs/
│       └── 2026-06-04-v1-enrichment-design.md
├── Dockerfile
├── go.mod
├── go.sum
└── README.md
```

### 12.2 关键决策

| 决策 | 选择 | 理由 |
|---|---|---|
| Go 包布局 | 根 `main.go` + `internal/converter/` 子包 | 单二进制小项目无需引入 `cmd/server/`；`internal/converter` 形成清晰的 API 边界，便于聚焦测试与未来复用 |
| 静态资源 | `//go:embed static/*` 用 `embed.FS` 编入二进制 | 编译产物是单文件可执行；Docker 镜像不再需要 COPY 静态目录；部署一致性更高 |
| CSS 共享 | 抽 `static/shared/theme.css` 用 `<link>` 引用 | index.html、docs.html 共用同一份主题变量，避免漂移 |
| 主题 bootstrap 脚本 | 仍内联各页 `<head>` | 必须 paint 前同步执行，外部脚本会有 FOUC |
| 错误码 | `internal/converter/errors.go` 单点定义并导出 | handler 层引用同一组常量字符串，避免硬编码漂移 |

### 12.3 `main.go` 职责收窄

重构后 `main.go` 只做三件事：

1. 创建 gin 引擎，用 `embed.FS` 挂载 `/static`、`/docs`、`/docs/spec` 静态路径
2. 注册四个 API 路由（`/api/convert` 等）并转发到薄 handler；handler 解 JSON、调 `converter` 包、写响应
3. 启动 `:8080` 监听

业务逻辑（转换、解析、校验、批量）全部下沉到 `internal/converter`，主包不再持有任何金额相关的字符串常量。

### 12.4 与 §4.1 的关系

本节**取代** §4.1 的扁平文件清单：不只是把文件拆出来，而是把转换逻辑整体放进 `internal/converter` 子包，让主包成为真正薄的 HTTP 适配层。测试文件随源码一起搬进同一包内（Go 惯例）。

### 12.5 变更范围

- **新增**：`internal/converter/` 整包；`static/shared/theme.css`；`static/docs.html`、`static/swagger.html`、`static/openapi.json`
- **修改**：`main.go` 重构为薄 handler + `go:embed`；`static/index.html` 改用 `<link>` 引用共享 CSS、加入 Tab 与四模式工作区；`Dockerfile` 移除单独 COPY static 的步骤
- **迁移**：现有 `ConvertToChinese` 函数与 `digits` / `units` 常量 → `internal/converter/forward.go`
- **删除**：根包内原有的转换逻辑与单位字符串常量（迁移后移除）
