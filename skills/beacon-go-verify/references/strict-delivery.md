# 严格交付协议

本协议由七个能力共同引用。implement 执行，verify 独立验收，goal 记录推进；
plan/truth/change/release 只执行本阶段适用部分。需求权威仍是 canonical intent tree。
这是宿主执行合同；文本检查通过不代表宿主已执行，也不提供文件系统权限隔离。
树叶 `verify leaf` 的运行时证据格式见仓内 `docs/sdd-truth.md` 的
“树叶运行时回执（beacon-delivery.v1）”；旧式文字日志不能单独升级 verified。

## 1. Plan-First：确认后实施

顺序为计划与原子拆解 → 确认 → 测试与实现 → 验证 → 独立审查 → 核销。
计划写明完成定义、输入输出、边界、AC、验证命令和任务依赖。
计划确认前只允许读取、调查和撰写计划材料，不写产品代码。
每次推进一个原子任务，通常限定一两个函数；超出时先拆解或记录必要的跨文件原因。
原子任务不等于自动 git commit；提交、推送、发布仍需用户授权。
方案说明属于 plan 产物，不增加 design 阶段、命令或 gate。

## 2. Red-Green：测试先行与断言保护

行为变更先写测试并实跑 Red，再实现并实跑 Green；记录同一测试的两次命令、
退出码、日志与候选版本。Red 必须针对预期缺失行为失败；环境故障、零测试、
跳过测试或无关编译错误不能冒充有效 Red。
Green 后重跑关联回归，最终候选执行项目规定检查。
实现阶段保持已确认断言不变；删除、跳过或弱化断言不能换取通过。
测试自身有误时，单独记录原因与修订差异，重新审查并重建 Red/Green 证据。
文档、纯配置或不适合 TDD 的任务记录不适用原因及等价验证；不伪造 Red。

## 3. Blind Review：全新上下文只读盲审

宿主启动独立 reviewer，仅传原始需求、固定候选的 git diff 和运行日志，
不传实现者推理或自评。审查者可只读检查相关源码、测试和依赖，必要时复跑验证。
检查遗漏边界、针对测试的硬编码、占位符、断言弱化、无关修改及 AC 覆盖。
回执记录 reviewer、候选 commit/diff hash、需求指纹、结论和未关闭问题。
实现者与审查者身份/会话不同；换名字或自审不等于独立审查。
宿主不支持独立 reviewer 时记录阻塞，可汇报进度但不能宣称独立验收完成。
内核不启动 agent、不调用 LLM；身份字符串本身不是身份认证。

## 4. Evidence：真实退出码与候选绑定

按项目 AGENTS/CI 执行静态/类型检查、构建、测试与必要的集成检查。
beacon-go 仓库使用 `go build ./...`、`go test ./... -count=1`、`go vet ./...`、
`bash scripts/check_version_consistency.sh`、`bash scripts/check_kernel_deps.sh`；
端到端链路变更另跑 `bash e2e/run_e2e.sh`。其他项目读取自己的命令配置。
任一必需检查退出码非零、超时、取消或未执行均阻止完成声明；保存真实 stdout/stderr。
扫描新增 diff 中 TODO/FIXME/NotImplementedError、空实现及调试残留；
按语法和上下文判断，合法 pass、展开运算符、测试夹具中的关键字不作占位实现。
真实占位实现必须修复；扫描规则误报要记录位置与依据，不能静默忽略。
证据绑定 source commit（dirty 候选另记 diff/input hash）、binary SHA 和需求指纹。
日志由实际命令采集；手写“exit 0”与文件 hash 不能证明命令执行或行为正确。
证据登记、hash 校验、tree verified、gate pass、version ready 分别汇报。

## 5. Ledger：外部状态逐项核销

以已确认 tasks.md/DAG 为任务定义，运行回执为执行事实；Markdown 清单与
progress.json 若用于展示，必须声明从该事实派生，不能产生第二账本。
通过已有 `implement tasks run/status` 或项目确定性脚本记录 task ID、结果和证据引用。
每次恢复先读取账本，核对需求指纹、候选和未关闭任务。
仍有未勾选项、blocked/failed 任务或缺证据时，只报告进度与阻塞，不宣称全部完成。
不靠手动勾选把任务变为通过；定义变更走 change 与重新确认。

## 6. Deterministic Tooling：脚本代替模拟

优先调用现有 CLI/项目脚本，复杂解析、Git 操作和环境配置封装为可审计程序；
使用前读取真实 --help/API 签名，按实际接口传结构化参数，不编造 flag。
脚本与版本一起维护，保留非零退出码、stdout/stderr 和输入指纹。
脚本成功只证明其检查范围，不能用成功输出替代未实现功能或人工批准。

## 最终汇报

列出变更文件及原因、真实验证命令/输出摘要、未覆盖边界、剩余任务和独立审查回执。
未运行写“未验证”，不确定写“未确认”。允许在阻塞时结束进度汇报，禁止伪造完成。
