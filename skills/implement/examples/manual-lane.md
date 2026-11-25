# Manual lane: host executes, kernel records

Chain `full-delivery`.

1. `beacon goal admit --run-id <id>` returns `allow_implement=true`.
2. The host does the work in the admitted worktree and commits it.
3. `beacon implement report --run-id <id> --node <touchpoint> --by dev --artifact report.txt` records the work.
4. `beacon verify leaf --node <touchpoint> --artifact report.txt --by qa` registers the evidence.
5. `beacon verify rollup --node <parent>` verifies the parent only when every child is verified.

The kernel never claims a stage finished without the report and evidence.
