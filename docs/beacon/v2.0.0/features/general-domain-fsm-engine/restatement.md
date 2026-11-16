---
slug: general-domain-fsm-engine
confirmed_by: apple
confirmed_at: 2026-11-16T14:17:00+08:00
hash: fb8ff0420f746537d541acbd41659a717f15af75ec84dd2a165eb5086cd6b4cc
---
## 1 交付形态

1. 在 `beacon-go` 内核源码中解耦状态机审查与新增 CLI 接口：
   - 修改 `internal/gate/intent.go` 与 `internal/gate/truthmd.go`：支持 `domain_kind: platform | workflow | general`，非电商包不索要用户旅程。
   - 新增 `cmd/truth_fsm.go`（或在 `cmd/truth.go` 扩展）：实现 `beacon truth fsm check`，输出实体依赖序、状态转移矩阵与非法跃迁覆盖度。
   - 新增 `cmd/fsm.go`：实现 `beacon fsm transition`，运行时判定状态跃迁合法性。
2. 单元测试用例：
   - 编写 `internal/gate/domainfsm_test.go` 与 `cmd/fsm_test.go`。

## 2 界面与交互

CLI 交互：
- `beacon truth fsm check --project <path> --feature <slug> [--json]`
- `beacon fsm transition --project <path> --feature <slug> --from <S1> --to <S2>`

## 3 业务流转

1. 用户或 Agent 在编写状态机特性时，显式标明 `domain_kind`。
2. `truth freeze` 期间，内核自动识别并进行严谨图结构校验，且豁免非电商包的旅程限制。
3. 调度器或应用在运行时执行 `beacon fsm transition` 进行动态裁决。

## 4 边界与不做

- 不硬编码任何项目（如 openmausbot）特有的状态名词。
- 不破坏现有电商特性包（`domain_kind: business`）的严苛 DDD 门禁。
- 保持向前兼容。

## 5 歧义与默认值

| 歧义 | 默认值 | 理由 |
|---|---|---|
| domain_kind 缺省时的行为 | 沿用既有启发式推断 | 保持老包兼容 |
| 非法转移判定的输出格式 | 默认输出简明终端文本，`--json` 输出完整结构 | 兼顾人读与脚本自动化调用 |
