# SentinelGate SQL 注入检测内核

Go 库，输入原始 URL 查询串（不含开头的 `?`），输出风险命中及证据在原始查询串中的字节区间。规则与工作数据仅在内存中，单线程调用。

## 用法

```go
hits, err := sentinelgate.Check("a=%2527%20UNION/**/SELECT")
// hits[0] = {Rule: RuleUnionSelect, ParamIndex: 0, Start: 10, End: 25}
// 证据片段 = query[10:25] = "UNION/**/SELECT"
```

- `Check(query string) ([]Hit, error)`：返回全部命中；查询串超过 1MiB 返回 `ErrQueryTooLarge`。
- `Hit`：规则类别（`RuleUnionSelect` / `RuleOrEquals` / `RuleExecComment`）、参数出现序号（从 0）、原始字节区间（左闭右开）。

## 检测规则

1. 相邻词元构成 `UNION SELECT` 或 `UNION ALL SELECT`（忽略 ASCII 大小写）。
2. `OR`、整数、`=`、整数，两整数数值相等（前导零不影响，不受机器位宽限制）。
3. 引号外以 `/*!` 开头的可执行块注释，无条件命中整个块。

## 处理管线

- **参数划分**：按 `&` 分段，按首个 `=` 分名值；只检查值，解码产物不重新划分参数。
- **两轮解码**：首轮 `+` 变空格再解 `%XX`；第二轮只解 `%XX`；无效转义逐字节保留。
- **词法**：假定始于单引号字符串内；字符串/注释内容不命中；同类双引号转义、单双引号支持反斜杠转义；`#` 与 `-- ` 行注释、`/* */` 块注释等价于空白；未闭合字符串或注释屏蔽至末尾。
- **证据回指**：每个解码字节携带原始区间，命中区间从首个词元原始起点到末个词元原始终点。

## 测试

```sh
go test ./...          # 功能 + 1MiB 对抗输入性能验证（<2s、<128MiB）
go test -bench=. .     # 吞吐基准
```
