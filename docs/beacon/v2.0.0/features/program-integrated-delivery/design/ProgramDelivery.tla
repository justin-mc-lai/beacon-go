---- MODULE ProgramDelivery ----
EXTENDS Naturals, FiniteSets

VARIABLES claims, qa, integrated, revision, tested, approved, closed
vars == <<claims, qa, integrated, revision, tested, approved, closed>>
Lakes == {"A", "B", "C", "D"}
Deps(l) == CASE l = "A" -> {} [] l = "D" -> {"B", "C"} [] OTHER -> {"A"}

Init == /\ claims = {} /\ qa = {} /\ integrated = {}
        /\ revision = 1 /\ tested = 0 /\ approved = 0 /\ closed = {}
Claim(l) == /\ Deps(l) \subseteq integrated /\ l \notin claims
            /\ claims' = claims \cup {l}
            /\ UNCHANGED <<qa, integrated, revision, tested, approved, closed>>
Verify(l) == /\ l \in claims \ qa /\ qa' = qa \cup {l}
             /\ UNCHANGED <<claims, integrated, revision, tested, approved, closed>>
Integrate(l) == /\ l \in qa \ integrated /\ integrated' = integrated \cup {l}
                /\ UNCHANGED <<claims, qa, revision, tested, approved, closed>>
ChangeCandidate == /\ integrated = Lakes /\ revision = 1 /\ closed # Lakes
                   /\ revision' = 2 /\ closed' = {}
                   /\ UNCHANGED <<claims, qa, integrated, tested, approved>>
AggregateQA == /\ integrated = Lakes /\ tested # revision /\ tested' = revision
               /\ UNCHANGED <<claims, qa, integrated, revision, approved, closed>>
Approve == /\ tested = revision /\ approved # revision /\ approved' = revision
           /\ UNCHANGED <<claims, qa, integrated, revision, tested, closed>>
Close(l) == /\ approved = revision /\ l \in Lakes \ closed /\ closed' = closed \cup {l}
            /\ UNCHANGED <<claims, qa, integrated, revision, tested, approved>>
Idle == UNCHANGED vars

Next == \/ \E l \in Lakes : Claim(l) \/ Verify(l) \/ Integrate(l) \/ Close(l)
        \/ ChangeCandidate \/ AggregateQA \/ Approve \/ Idle
Spec == Init /\ [][Next]_vars
NoPrematureClaim == \A l \in claims : Deps(l) \subseteq integrated
NoUntestedIntegration == integrated \subseteq qa
NoUnsignedClose == closed # {} => (approved = revision /\ tested = revision /\ integrated = Lakes)
====
