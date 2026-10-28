# M-D 收口判定表（20 条三选一）

> 日期：2026-10-26 ｜ 仓：`beacon-go` ｜ HEAD：`630d96b` ｜ 作者：Beacon 产品
> 输入：`docs/deferred-external.md`（A 段 16 + B 段 4）、`docs/ac-matrix.md`（171 行，tested 82 / implemented 64 / deferred-external 16 / dropped 9）
> 性质：**提案，不是判定**。分类列（`docs/ac-matrix.md` 第 4 列）一字未动；改判需用户拍板。
> 判定口径：全部 20 条的验证命令于 2026-10-26 09:36 本机重跑，判定依据是今天的实测输出，不是 10-25 的旧结论。

## 一、先说结论：16 条里只有 4 条真的在等外部，另 1 条是在等你拍板

把 16 条按「谁欠这个条件」重新归因，不是一个桶：

| 归因 | 条数 | AC |
|---|---|---|
| 真等外部（样本/账号/工具不存在） | 4 | AC-009、AC-039、AC-046、AC-078 |
| 等用户拍板（阻塞物是决策未下，不是外部条件） | 1 | AC-157 |
| 前置已满足，缺的是我们自己写代码 | 3 | AC-043、AC-045、AC-077 |
| **本不该进这张表**（纯代码或纯产品缺口） | 8 | AC-092、AC-095、AC-105、AC-111、AC-112、AC-114、AC-127、AC-144 |

含义：M-D 原来记作「16 条等外部、只能等」，实际是「4 条真等、1 条等拍板、11 条今天就能动」。
「等外部」和「等拍板」不能混记：前者我们动不了，后者是一句话就能解锁的决策，混在一起会让能推进的项看起来像在等别人。
这也是 M-D 迟迟收不了口的机制原因——把「没写」和「没定」都记成了「等外部」，于是没人认领。

## 二、A 段 16 条逐条三选一

| AC | 今日实测（原命令） | 三选一 | 依据 |
|---|---|---|---|
| AC-009 | `find . ~/.beacon -name 'loop-runs.db'` → 无输出；`scan.go:254` 仅登记候选路径 | **长期挂起** | 真外部：全机无 v1 loop 样本，schema 无从探明 |
| AC-039 | `grep -rl "evidence-signature-chain" internal/` → 0 | **长期挂起** | 真外部：无 v1 签名链文件样本 |
| AC-043 | `codex --version` → `codex-cli 0.156.1`；`Test.*Codex` → 0 | **已具备 → 转实现** | 二进制已装已登录，缺的是集成测试本身，属代码 |
| AC-045 | `claude --version` → `2.1.281 (Claude Code)`；`Test.*Claude` → 0 | **已具备 → 转实现** | 同上，缺集成测试，属代码 |
| AC-046 | `grep -rn acp_full_backend_not_supported internal/` → 2 | **长期挂起** | 真外部：需装一个真 ACP agent，当前仅 initialize + session/new 两步探针 |
| AC-077 | `~/.beacon/runtime-ledger/runtime-ledger.sqlite3` **存在**（219MB，SQLite 3，16 张表已可读）；`scan.go:254` 只列 uncertain 不导入 | **已具备 → 转实现** | 原前置「出现以探明 schema」已满足：文件在、schema 可读，剩下是导入逻辑与 v1/v2 形态判定，属代码 |
| AC-078 | `find` 无 `loop-runs.db` | **长期挂起** | 真外部：无 v1 loop 样本 |
| AC-092 | `grep -rn backup internal/migration internal/cli/migrate.go` → 0 | **移出本表（纯代码，P1）** | 不依赖任何外部条件；本表 §D 自己也这么写，却仍列在 A 段 |
| AC-095 | `bench/results-go5-baseline.json` **存在**；`run_bench.sh` 里 `baseline` 命中 0 | **移出本表（纯代码）** | 基线已有、回退门禁无；矩阵原备注「基线本身未建」已过期 |
| AC-105 | `grep -rn "signal.Notify" internal/` → 0 | **移出本表（纯代码）** | 进程内 loop / checkpoint-resume 已有测试，缺的只是 SIGINT 处理代码 |
| AC-111 | `grep -rn intent_snapshot internal/` → 3，全是 reason 码与 JSON 字段名 | **移出本表（产品决策）** | 无 v1 产物；阻塞物是「七字段集与落盘形态由谁定」，责任角色是我，不是外部 |
| AC-112 | `decision_packet\|prompt_snapshot` → 0 | **移出本表（产品决策）** | 同 AC-111 前置，属设计未定，非外部未到 |
| AC-114 | 同上 → 0 | **移出本表（产品决策）** | 同 AC-111 |
| AC-127 | `go:embed goal plan truth design implement qa release` 已在 `skills/skills.go:25`；`find skills -name '*.md'` → 15；`examples` 命中 0 | **移出本表（文档缺口）** | 嵌入已完成，矩阵原备注「未嵌入、仍是占位」已过期；缺的只是 route examples 正文 |
| AC-144 | 同 AC-127 | **移出本表（文档缺口）** | 同 AC-127 |
| AC-157 | `RuntimeTrack\|runtime_track_unspecified` → 0 | **长期挂起** | 等拍板（不是等外部）：选轨面（独立名或 `--runtime`）要用户定，定了才能动；附带需 CI 三矩阵 |

**分布：长期挂起 5 ｜ 已具备转实现 3 ｜ 移出本表归因改写 8 ｜ 降 dropped 0。**

## 三、B 段 4 条外部验证项

| 项 | 今日实测 | 三选一 | 依据 |
|---|---|---|---|
| ACP 真迭代（session/prompt） | `acp_full_backend_not_supported` → 2 | **长期挂起** | 与 AC-046 同一阻塞，同一件事不重复计数 |
| 真 codex / claude 二进制多轮 | 两个二进制均在位 | **已具备 → 转实现** | 与 AC-043/045 同一件事，二进制不再是阻塞物 |
| Chrome 扩展 e2e | `command -v google-chrome \|\| chromium` → MISSING | **长期挂起** | 矩阵无对应 AC 行，等宿主 |
| loop token 账本 | `TokenUsage` → 2 处字段在，子进程恒 0 | **长期挂起** | 阻塞物是宿主回报通道，不是代码 |

**分布：长期挂起 3 ｜ 已具备转实现 1 ｜ 降 dropped 0。**

## 四、我不建议做的两件事

1. **不建议翻分类桶**。11 条若从 `deferred-external` 挪进 `implemented`，会把「代码已实现但未测」和「根本没写」混成一类，171 行的四类分布立刻失去区分力（implemented 64 → 75 就不再是同一个意思）。建议**分类列不动，只改「阻塞对象」列的归因**：把「外部条件未到」改写成「代码未写 / 产品未定 / 外部未到」，M-D 就按「谁欠」推进，计数口径不受污染。
2. **不建议现在降 dropped**。20 条里没有一条是本轮明确不做的：AC-157 等一个人拍板、AC-009/039/078 等一份历史样本、ACP 等装一个 agent、Chrome 等一个宿主。真正可以判 dropped 的时机是用户明说「这条不做」，而不是「等了很久」。

## 五、需要拍板的三件事

1. 11 条改归因（分类列不动）是否照此执行？只改 `docs/deferred-external.md` 的「前置条件/阻塞对象」两列 + 矩阵备注，不动分类列。
2. AC-043 / AC-045 / AC-077 三条前置已满足，是否要我出「转实现」的验收标准（AC-077 尤其需要先定：那个 219MB 文件到底算 v1 样本还是 v2 全局库——这决定 AC-077 是真待办还是已被 AC-171..173 吸收）。
3. AC-111 / AC-112 / AC-114 / AC-127 / AC-144 五条卡在我身上（字段集、decision_packet 形态、route examples 正文）。要不要我在 truth 冻结里一次性定字段集，把它们从「待复核」变成有验收标准的待办。

## 六、边界

- 本表未改任何 AC 分类，未改 `docs/ac-matrix.md` 一个字节。
- 全部判定可复跑：第二节每行的命令即验证命令，输出随本机环境变化。
- 未 commit。
