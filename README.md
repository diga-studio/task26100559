# SentinelGate SQL 注入检测内核

Go 库，由上层直接调用。输入原始 URL 查询串（不含开头的 `?`），输出风险
命中及证据在**原始查询串**中的字节区间。规则与工作数据仅保存在内存中。

## 用法

```go
import "sentinelgate"

hits := sentinelgate.Check("a=%2527%2520UNION/**/SELECT")
for _, h := range hits {
    // h.Rule: RuleUnionSelect / RuleOrEquals / RuleExecComment
    // h.ParamIndex: 参数在查询串中的出现序号（0 起）
    // h.Start, h.End: 原始查询串字节区间，左闭右开
}
```

## 检测模型

- 按 `&` 切参数、按首个 `=` 分名值，只检查值；解码产生的分隔符不会
  重新划分参数。
- 每个值恰好解码两轮：第一轮 `+` 变空格并解码 `%XX`，第二轮只解码
  `%XX`；无效转义逐字节保留，无第三轮、无 Unicode 归一化。
- 词法从"已位于单引号字符串内"开始；引号字符串与反引号标识符内容不
  命中、不拼合词元；普通注释等价于空白；未闭合字符串/块注释屏蔽至
  末尾但不撤销此前命中。
- 三类规则：`UNION [ALL] SELECT` 相邻词元；`OR <int> = <int>` 等值
  （忽略前导零、不受机器位宽限制）；引号外 `/*!...*/` 可执行注释。

## 测试

```sh
go test ./...                                  # 全部单元与验收测试
go test -run TestCheckAdversarial1MiB -v .     # 1MiB 对抗输入耗时/内存
go test -bench=. -run=NONE .                   # 基准
```

参考数据（Apple M1 Max）：1MiB 对抗输入单次检查约 15–30ms，额外分配
约 42MB，远低于 2s / 128MiB 限制。
