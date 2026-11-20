---- MODULE ProgramDeliveryBuggy ----
EXTENDS ProgramDelivery
EarlyClaim == /\ "B" \notin claims /\ claims' = claims \cup {"B"}
              /\ UNCHANGED <<qa, integrated, revision, tested, approved, closed>>
BugSpec == Init /\ [][Next \/ EarlyClaim]_vars
====
