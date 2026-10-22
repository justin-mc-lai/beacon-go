# P6 修复：写路径性能（合并落盘）+ v1 QA 慢因分析

版本：v2.0.0-go.6（性能修复版）。日期：2026-10-22。
用户拍板口径：「修，性能提高，修法是合并落盘」——本报告即该项的交付记录。

## 1. 结论先行

- **合并落盘已完成**：store 新增事务批处理通道 `WithTx`，所有多写路径
  （过门评估、开跑、推进一格、检查点、完成、司机单轮等）从「每写一行
  落一次盘」改成「一整批写一次落盘」。写的行数、事件内容一条没少
  （210 个测试 + E2E 全链双证明，见第 6 节）。
- **benchmark 8 项过 7**（改造前过 5）：迁移 25.2 秒 → 1.76 秒（约 14 倍）；
  推进一格 p95 176ms → 97.8ms 过线；单文件哈希校验转过。只剩过门评估
  （AC-054）仍超阈，原因不在写多写少，而是这台虚拟机的磁盘「同步」
  动作本身慢且飘（第 2、5 节给实测账）。
- **synchronous 改为 NORMAL**（只对可写打开生效）：代价写清在第 4 节——
  数据库永不损坏、程序崩溃不丢数据；只有操作系统崩溃/断电可能丢掉
  最后已提交的一小批，而 beacon.db 存的是可重建的运行状态。
- **v1 的 QA 为什么慢**：哈希确实是锅之一，但不是最大的（第 7 节独立分析）。

## 2. 定位实测：慢路径的落盘账（改造前逐处数过）

数据库走 SQLite WAL 模式；改造前每个 DAO 写都是一次独立自动提交 =
一次独立落盘（WAL 同步）。三条慢路径的提交次数：

| 路径 | 改造前每次调用的落盘次数 | 明细 |
|---|---|---|
| gate evaluate（persistGateResult） | 2 | gate_results 一行 + lifecycle_events 一行 |
| goal tick 推进成功 | 4 | 执行器事件 + lifecycle 更新 + stage_advanced 事件 + run 状态 |
| goal tick 被拦（admission/exit 两分支） | 4 | applyBlock 的 3 次 + 一次与之内容相同的重复 saveRun |
| goal run（CreateRun） | 4 | run 行 + 创建事件 + run 状态 + lifecycle |
| Checkpoint | 3 | 检查点行 + 事件 + run 状态 |
| Complete 成功收尾 | 3–5 | 确认事件+状态（2，仅确认时）+ 完成事件 + 状态 + lifecycle |
| driver 单轮收尾 | ≈5 | turn_end 事件 + 状态 + Checkpoint 的 3 次 |
| migrate v1 导入 | 每包 1 个事务（已是批量） | 慢在每包一次 FULL 同步 × 128 包 |

单价实测（探针，与默认库同卷的 btrfs home 卷，临时探针文件已删，
原始输出存 `bench/probe-output.txt`）：

| 测量 | synchronous=FULL（改造前默认） | synchronous=NORMAL |
|---|---|---|
| 一次单行提交 | median 23.0ms | — |
| 一个事务装 1 行 / 4 行 | 22.4ms / 23.8ms | 4 行 0.10ms |
| AppendEvent 一次 | median 31.3ms | 0.07ms |
| 只开库+关库（可写） | 2.1ms | 2.1ms |
| 开库 + 写 1 行 + 关库全程 | ≈72ms | ≈72ms（见下） |
| 开库 + 一批 2 行 + 关库全程 | — | ≈71ms（第二行免费） |

三个关键事实：

1. **贵的是「提交的次数」，不是「写的行数」**：FULL 下一个事务装 1 行
   还是 4 行价格一样（约 23ms），所以把 N 次提交并成 1 次就是 N 倍省。
2. **这台虚拟机的磁盘同步（fsync）慢且飘**：`dd oflag=dsync` 写 4KB
   实测单次同步 14–19ms（设备时间），且随宿主机负载在约 2ms 到 45ms
   之间大幅波动。同一条 evaluate 命令在「快天气」测过 median 7.6ms、
   在「慢天气」是 88ms——数字是天气报告，结论必须看结构（提交次数）。
3. **NORMAL 之后仍有约 70ms 的「慢天气地板」**：strace 计时显示一次
   写命令全程约 4 次 fsync（开 WAL、提交、关库时把 WAL 合并回主库
   并同步），CPU 时间只有约 18ms。关库合并 WAL 的同步是 SQLite 的
   生命周期行为，任何 WAL 应用都躲不掉；在本地 SSD（如用户 MacBook，
   fsync 约 0.1–1ms）上这部分只有几毫秒。同一个库放 tmpfs 上跑同一条
   命令全程 14ms，可作对照。

## 3. 改造清单

**store 层**（`internal/store/tx.go`，新增）：
- `Store.WithTx(fn)`：批处理入口。进入先查写权限，fn 出错全回滚，
  成功一次提交；嵌套调用直接拒绝（`store_transaction_nested`）——
  曾踩过坑：单连接下嵌套事务会死锁，守卫必须在任何数据库语句之前。
- `Tx` 句柄提供与 Store 同名的 DAO 访问器（Projects/Features/Runs/
  Gates/Artifacts/AppendEvent），批内写入走同一批事务。
- DAO 全部改为走「查询接口」：单写 API 签名与语义不变，未改造的
  调用方零影响；`AddVersion/SetCurrent` 在批内并入调用方事务、
  批外维持自开事务（原语义）。

**调用方合并**（提交次数 改造前 → 改造后）：

| 路径 | 前 | 后 |
|---|---|---|
| gate evaluate / integrity（persistGateResult） | 2 | 1 |
| goal run（CreateRun） | 4 | 1 |
| tick 推进成功 | 4 | 2（执行器事件保持独立提交，见残留项） |
| tick 被拦 / 失败 / repair 路径 | 3–4 | 1 |
| Checkpoint | 3 | 1 |
| Heartbeat / RecordInterrupt / Continue | 2 | 1 |
| Complete（确认对 / 收尾三写 / AttachVerifier / Block） | 2–3 | 1 |
| driver 单轮收尾（turn_end + 状态 + 检查点三写） | 5 | 1 |
| driver 停机 / 预算耗尽 / release 早停 | 2 | 1 |
| 技能评分（Score 行 + 事件） | 2 | 1 |

顺带修掉的真重复写（最终落盘内容与原来逐字一致，仅去掉冗余动作）：
tick 的 admission / exit 两分支在 applyBlock 已保存同一份状态后，
又多写了一次完全相同的 saveRun——已删。

**原子性收紧（两处，属合并的必然结果，成功路径行为不变）**：
- tick 推进：原来 lifecycle 先单独落盘、之后内存里的阶段机若报错会
  留下半更新状态；现在整批要么全落要么全不落。
- driver 单轮：原来检查点失败时轮末状态已落盘；现在整轮收尾一起
  回滚（错误照样上报，命令结果不变）。

**没动的**：planner_review 的持久化（中间夹文件落盘，不把文件 I/O
关进事务）；spine 派生、doctor rebuild 等读路径；所有阈值与基准
脚本口径（`bench/run_bench.sh` 一字未改）。

## 4. synchronous 取舍：FULL → NORMAL（仅可写打开）

A/B 实测（同一脚本、同一台机器、背靠背三轮，见第 5 节表）：合并之后
FULL 仍过不了线（AC-054 p95 95.8、AC-057 p95 111.3）；换 NORMAL 后
提交单价从约 23ms 掉到约 0.1ms，迁移从 21.6 秒掉到 1.5 秒。

耐久性代价（写清，按 SQLite 官方语义）：
- WAL 模式下 NORMAL **不会损坏数据库**，任何情况下都不会；
- **程序崩溃不丢数据**（已提交事务在 WAL 里，下次打开自动恢复）；
- 只有**操作系统崩溃 / 断电**这一种情况，可能丢掉最后已提交的
  若干事务（回滚到上一个同步点）。
- 本项目的可接受性：beacon.db 是运行状态不是真相本体——真相在
  Markdown 文件里，`doctor rebuild` 可整库重建；门的结论可重评；
  run 状态有检查点机制。丢最后一小批运行记录不丢任何真相。
- 落点：可写 DSN 加 `_pragma=synchronous(NORMAL)`（`store.go`
  sqliteDSN），只读打开不受影响；并有测试钉住该值
  （TestWritableStoreUsesSynchronousNormal），将来谁改回去测试会响。

考虑过但**拒绝**的更激进做法：命令退出时不关库、把 WAL 留在盘上
（下次打开靠恢复）——省的是关库合并的那几次同步，但会留下残留
文件、换机拷贝易丢数据，为环境相关的收益冒这个险不值。

## 5. Benchmark 新旧对照（`bench/run_bench.sh` 原样，阈值未动）

| AC | 项 | 阈值 | 基线 go.5 | 合并+FULL | 合并+NORMAL | 终版 go.6 | 判定 |
|---|---|---|---|---|---|---|---|
| 051 | 冷启动 | p95<100 | 3.88/4.70 | 4.04/5.03 | 4.47/10.94 | 4.43/5.39 | ✅ |
| 052 | spine show | p95<50 | 9.38/17.61 | 8.53/10.16 | 8.42/14.55 | 9.04/11.00 | ✅ |
| 053 | truth show | p95<200 | 9.02/11.86 | 7.91/8.77 | 8.11/12.27 | 9.59/23.56 | ✅ |
| 054 | gate evaluate | p95<50 | 108.71/132.29 | 81.46/95.79 | 81.71/99.23 | 84.23/121.45 | ❌ |
| 055 | 单文件哈希校验 | p95<10 | 7.13/13.12 | 6.61/8.78 | 6.56/9.31 | 6.05/7.13 | ✅ |
| 056 | migrate v1（秒） | <60 | 25.22 | 21.65 | 1.51 | 1.76 | ✅ |
| 057 | goal tick | p95<100 | 150.26/176.22 | 102.38/111.29 | 84.40/101.80 | 84.70/97.82 | ✅ |
| 058 | 库体积 | <500MB | 0.41MB | — | — | ✅ | ✅ |

（每格为 median/p95，单位 ms；终版原始结果 `bench/results.json`，
A/B 两轮分别存 `bench/results-merged-full.json` 与终版前的
`bench/bench-merged-normal.log`，基线存 `bench/results-go5-baseline.json`。）

**AC-054 为何还超**：过门评估的只读部分（gate status）同轮实测
p95 14.3ms，轻松过线；evaluate 比 status 多的只有「写一行结论 +
一条事件」——已并成一个事务。剩下的耗时就是第 2 节的地板：一次
写命令绕不开的 WAL 生命周期同步（约 4 次 × 本机当前 15–25ms）。
终版轮次里 evaluate 的最小样本都是 72ms，说明整轮都处在慢天气；
换到本地 SSD，这条命令的实测量级是十几毫秒（tmpfs 对照 14ms）。
结论：**能合并的已经合并干净，最后这一项卡
...[truncated 3439 chars]