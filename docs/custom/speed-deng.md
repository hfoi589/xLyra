# 雷速蹬统计模式

雷速蹬是一个只影响统计采集的 Codex OAuth 自定义扩展。它不改变网关路由、OAuth token、上游请求或源项目的 `request_logs` / `usage_records`。

## 状态接口

- `GET /api/v1/settings/speed-deng`
- `POST /api/v1/settings/speed-deng/start`
- `POST /api/v1/settings/speed-deng/stop`

任一管理员开启后为全局状态。服务重启会恢复活动会话；启动检查或每分钟额度检查发现任一启用 Codex OAuth 账号的 `weekly.remaining_percent > 99` 时，只停止雷速蹬采集。

## 独立数据

数据写入同一 PostgreSQL 中的独立表：

- `custom_speed_deng_sessions`
- `custom_speed_deng_events`

事件在源请求记录提交成功后以独立事务写入。事件写失败不会回滚源请求，也不会改变客户端响应。事件仅记录成功且有 Tokens 或成本的 Codex OAuth 请求，并以源请求日志 ID 去重。

## 费用分摊

费用分摊接口先读取源项目用量，再读取雷速蹬事件。普通下游密钥名称保持原归集规则；雷速蹬项在归集名称后追加 `-雷速蹬`，例如 `Wilson-雷速蹬`。

额度和原始用量仍为 USD；`account_fee`、已分摊金额和未分摊金额为 CNY。计算公式保持不变：

```text
usage_ratio = usage_usd / total_quota_usd
allocated_cost_cny = usage_ratio * account_fee_cny
```

雷速蹬独立表读取失败时，费用分摊回退到源项目数据并返回可显示的告警。
