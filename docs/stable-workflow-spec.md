# 稳定工作流内核补强

用户确认范围：阶段完成合同、需求合同/执行快照分离、统一推进判断、存量需求归属合并、自动HTML与状态回执。
需求生成及语义执行仍归宿主；内核不调LLM、不启动agent。七能力保持不变。

## 本次执行合同

1. 新工作流显式绑定 project/version/feature/run 与工作流合同文件。合同定义用户目标、范围、模块、依赖、唯一canonical feature/leaf归属、原始AC来源覆盖和未知项。
2. 阶段合同按下表区分；检查返回当前run的缺项和下一责任人。truth的HTML通过既有RenderHTMLTree生成，绑定本轮scope与来源的合同hash，保留deprecated墓碑，不另写第二渲染路径。
3. read-only status/check不写文件/账本；显式prepare产生派生覆盖清单与HTML，必须另行confirm（身份+expected prepared hash）。tick/状态读取复用同一事实判定；prepare不等于确认或语义正确。实际CLI/schema与边界见[运行说明](stable-workflow-runtime.md)。
4. 工作流合同在run绑定时记录hash，修改必须重新显式绑定与确认；不从隐式latest run读取。新稳定模式与legacy模式必须明确显示，不把旧流程当稳定模式通过。
5. 测试选择器可采用go package+required_tests规则，运行时发现真实test_files并在capture记录，规范化只一个入口。测试清单变化不要求修改冻结Spec；同一Red/Green测试hash及最终候选校验仍严格，历史v1显式文件清单兼容。
6. 存量来源映射仅引用canonical node ID，重复调度归属或覆盖项丢失拒绝；prepare只预览合并映射，不改变冻结树、不自动选择重复合同的语义取舍。实际重组通过现有change/update/apply执行，需独立审阅。
7. truth/implement/verify/release继续复用现有freeze/admission/证据/review/人工批准；稳定合同检查不能替代原门禁。任何自动prepare或check都不执行LLM、发布、安装。

## 阶段表与持续树

| 阶段 | 完成合同 |
| --- | --- |
| plan | 调查、头脑风暴、分析，澄清目标、范围选项、来源与未知项；无需已有树或HTML，候选树可选。 |
| truth | 将本轮范围形成唯一canonical树，必交当前source hash绑定的HTML供审阅批准，自底向上冻结并保留真实回执。既有CLI plan apply只是工具命名，不倒推阶段。 |
| implement | 宿主经admission执行冻结范围的实现，内核不调LLM、不跑agent。 |
| verify | 独立验证者以真实证据验收本轮范围，区分hash校验、gate与树验证。 |
| release | 按本轮范围与依赖闭包评估，用户批准；不凭文档状态宣称发布。 |

逻辑一棵需求树持续成长，canonical JSON唯一权威；版本保存不可变baseline + 本轮delta。
已有树后续所有节点变更统一经change入口，包含draft调整；宿主按实际节点状态使用现有命令。
future draft可以同时存在，范围与依赖闭包外的草稿不拖本轮发布；本轮依赖不能靠改标签绕过。
此处是目标合同，内核若仍拒绝范围隔离或在plan要求HTML，必须报告实际阻塞，不冒充已经实现。
用户只决定目标、取舍、批准；宿主依内核next/missing处理技术映射、绑定、材料与命令，
不把重复绑定/清单交给用户解释。每阶段输出当前run/阶段/实际产物/下一步/owner。

## 成功与拒绝

实跑新建与存量映射、HTML自动生成、只读恢复、增加测试不改Spec、缺HTML/缺覆盖/重复归属/依赖环/陈旧文件拒绝。
阶段合同的结构/覆盖检查不证明模块划分符合第一性原理，必须交宿主独立审查。
Portfolio正式迁移暂停，当前材料作为存量测试输入，不批量冻结其208节点。
