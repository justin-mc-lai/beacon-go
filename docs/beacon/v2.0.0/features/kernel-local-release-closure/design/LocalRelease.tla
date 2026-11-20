---- MODULE LocalRelease ----
EXTENDS Integers
VARIABLES phase, approved, artifact, evidence, target, expected, backup, intent
vars == <<phase, approved, artifact, evidence, target, expected, backup, intent>>
Init == /\ phase = "prepared" /\ approved = FALSE
        /\ artifact = 1 /\ evidence = 1 /\ target = 0
        /\ expected = 0 /\ backup = -1 /\ intent = FALSE
Approve == /\ phase = "prepared" /\ artifact = 1 /\ evidence = 1
           /\ approved' = TRUE /\ phase' = "approved"
           /\ UNCHANGED <<artifact, evidence, target, expected, backup, intent>>
Begin == /\ phase = "approved" /\ approved /\ artifact = 1 /\ evidence = 1
         /\ target = expected /\ backup' = target /\ intent' = TRUE
         /\ phase' = "installing"
         /\ UNCHANGED <<approved, artifact, evidence, target, expected>>
Switch == /\ phase = "installing" /\ intent /\ artifact = 1 /\ evidence = 1
          /\ target = expected /\ target' = 1
          /\ UNCHANGED <<phase, approved, artifact, evidence, expected, backup, intent>>
Verify == /\ phase = "installing" /\ target = 1 /\ approved /\ intent
          /\ artifact = 1 /\ evidence = 1 /\ phase' = "installed"
          /\ UNCHANGED <<approved, artifact, evidence, target, expected, backup, intent>>
Tamper == /\ artifact = 1 /\ artifact' = 2
          /\ UNCHANGED <<phase, approved, evidence, target, expected, backup, intent>>
Drift == /\ target # 2 /\ target' = 2
         /\ UNCHANGED <<phase, approved, artifact, evidence, expected, backup, intent>>
Pause == UNCHANGED vars
Next == Approve \/ Begin \/ Switch \/ Verify \/ Tamper \/ Drift \/ Pause
Spec == Init /\ [][Next]_vars
ApprovedBeforeEffect == phase \in {"installing", "installed"} => (approved /\ intent /\ backup = expected)
NoSilentOverwrite == [][Switch => target = expected]_vars
====
