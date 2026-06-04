# 人民币大写转换工具

## 项目概述

使用 Golang + Gin + Vue 3 实现的人民币大写转换工具。支持 4 种模式：数字转大写、大写还原数字、双向校验、批量转换。

## 技术栈

- **后端**: Golang、Gin 框架、go:embed
- **前端**: Vue 3、Axios、自定义 CSS 变量主题（深色/浅色）
- **字体**: Bricolage Grotesque、JetBrains Mono、Noto Serif SC

## 快速开始

```bash
go run .
```

在浏览器中访问 `http://localhost:8080`

## API 文档

完整接口说明、代码示例与在线试调请见 `http://localhost:8080/docs`。
OpenAPI 规范：`GET /openapi.json` · Swagger UI：`/docs/spec`。

### 端点速览

| 方法 | 路径 | 用途 |
|------|------|------|
| POST | `/api/convert` | 数字 → 中文大写 |
| POST | `/api/convert/reverse` | 中文大写 → 数字 |
| POST | `/api/convert/verify` | 双向一致性校验 |
| POST | `/api/convert/batch` | 批量转换（最多 200 条） |

### 错误码

| HTTP | code | 说明 |
|------|------|------|
| 400  | `invalid_format` | 金额格式不符 |
| 400  | `out_of_range`   | 超出 999,999,999,999.99 上限 |
| 400  | `unparsable_chinese` | 中文大写无法解析（含 `at` 偏移） |
| 413  | `batch_too_large` | 批量超过 200 条 |

### 功能特点

- 4 种模式 Tab：转写 / 还原 / 校验 / 批量
- 深色 / 浅色主题切换（跟随系统，可手动覆盖）
- 反向解析宽容输入：圆 / 元、整 / 正 任一写法均可
- 负数支持（合同冲销 / 退款场景）
- 客户端预校验批量上限（粘贴超过 200 条立即提示）
- API 文档页内嵌 Try It 调试器，不持久化调试历史
- 单二进制 Docker 部署（静态文件通过 go:embed 内嵌）

## 构建

```bash
docker build -t rmb-server .
docker run --rm -p 8080:8080 rmb-server
```
