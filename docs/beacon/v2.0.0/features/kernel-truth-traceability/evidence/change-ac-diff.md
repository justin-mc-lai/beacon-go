# change-ac-diff.md

command: go test ./internal/orchestrator -run TestRefreezeRecordsACDiff -count=1
result: PASS

observed change_ac_diff: {
  "added": [
    "AC-004"
  ],
  "changed": [
    {
      "ac": "AC-002",
      "before": "275eed625288",
      "after": "1fee2ab76e4b"
    }
  ],
  "removed": []
}

change document section:

## AC Diff

added:
- AC-004

changed:
- AC-002 (before 275eed625288 → after 1fee2ab76e4b)

removed:
- (none)

