#!/usr/bin/env python3
"""Behavioural tests for scripts/local_release.py.

Run one suite at a time (the commands are the acceptance contract):
    python3 scripts/test_local_release.py manifest
    python3 scripts/test_local_release.py install
    python3 scripts/test_local_release.py status

No framework: plain asserts, a real temp filesystem per case, real
subprocesses for the concurrent-installer and crash-resume cases.
Every regression the human review found has a failing counterexample
here (evidence substring masking, empty/self checker, unbound binary,
per-state-dir locking, prepare wiping history, resume skipping checks,
optional preimage, unknown target overwrite, version-only probe).
"""
import json
import os
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
SCRIPT = os.path.join(HERE, "local_release.py")
sys.path.insert(0, HERE)
import local_release as lr  # noqa: E402

PASS = []

GOOD_BINARY = """#!/bin/sh
if [ "$1" = "--version" ]; then echo "Beacon CLI v9-candidate"; exit 0; fi
if [ "$1" = "plan" ]; then
  if [ "$2" = "--help" ]; then
    echo "init tree expand apply"; exit 0
  fi
  if [ "$2" = "tree" ]; then echo '{"error":"project not registered"}'; exit 1; fi
fi
exit 0
"""

# Answers --version (exit 0) but has no integrated entry: the old
# version-only probe would call this "usable".
BROKEN_BINARY = """#!/bin/sh
echo "Beacon CLI v9-broken"; exit 0
"""


def check(cond, msg):
    if not cond:
        raise AssertionError(msg)
    PASS.append(msg)


def write(path, text, mode=None):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(text)
    if mode:
        os.chmod(path, mode)


def make_binary(path, body=GOOD_BINARY):
    write(path, body, 0o755)


def evidence_text(checker, commit, bin_sha, verdict="PASS"):
    return ("# Independent QA report\n\n"
            "- verdict: %s\n"
            "- candidate: %s\n"
            "- binary: %s\n"
            "- checker: %s\n"
            "- tests: go test ./... -count=1 (exit 0)\n" % (verdict, commit, bin_sha, checker))


def isolated_env(root):
    env = dict(os.environ)
    env["BEACON_RELEASE_LOCK_DIR"] = os.path.join(root, "locks")
    return env


def setup_case(root, body=GOOD_BINARY):
    binary = os.path.join(root, "dist", "beacon")
    make_binary(binary, body)
    sha = lr.sha256_file(binary)
    commit = "a" * 40
    evidence = os.path.join(root, "qa-report.md")
    write(evidence, evidence_text("qa-bot", commit, sha))
    state = os.path.join(root, "state")
    return binary, evidence, commit, sha, state


def prepare(root, binary, evidence, commit, state, maker="maker-1", fix="integrity path fix"):
    return lr.main(["prepare", "--state-dir", state, "--feature", "kernel-local-release-closure",
                    "--version", "v2.0.0-go.15", "--requirements", "fp-req-1",
                    "--source-commit", commit, "--fix", fix,
                    "--build-cmd", "go build -trimpath -o dist/beacon ./cmd/beacon",
                    "--build-conditions", "darwin/arm64", "--binary", binary,
                    "--evidence", evidence, "--run-id", "run_1", "--maker", maker])


def digest_of(state):
    with open(os.path.join(state, "manifest.json"), encoding="utf-8") as f:
        return json.load(f)["digest"]


def approve(state, actor="lai", digest=None):
    return lr.main(["approve", "--state-dir", state,
                    "--manifest-digest", digest or digest_of(state), "--actor", actor])


def run_install(state, target, expect_old=None, env=None):
    argv = [sys.executable, SCRIPT, "install", "--state-dir", state, "--target", target]
    if expect_old is not None:
        argv += ["--expect-old-sha", expect_old]
    proc = subprocess.run(argv, capture_output=True, text=True, env=env)
    return proc.returncode, proc.stdout + proc.stderr


def make_target(root, body=None, label=None):
    target = os.path.join(root, "opt", "beacon")
    if body is not None:
        make_binary(target, body)
    elif label is not None:
        make_binary(target, '#!/bin/sh\necho "%s"\n' % label)
    return target


# ------------------------------------------------------------------


def suite_manifest():
    root = tempfile.mkdtemp(prefix="lr-manifest-")
    try:
        binary, evidence, commit, sha, state = setup_case(root)
        check(prepare(root, binary, evidence, commit, state) == 0, "prepare binds a valid candidate")
        digest = digest_of(state)
        with open(os.path.join(state, "manifest.json"), encoding="utf-8") as f:
            doc = json.load(f)
        check(doc["source_commit"] == commit and doc["binary"]["sha256"] == sha,
              "manifest binds source commit and binary sha")
        check(doc["evidence"][0]["sha256"] == lr.sha256_file(evidence), "manifest binds evidence hash")
        check(doc["fixes"] == ["integrity path fix"] and doc["runs"] == ["run_1"],
              "manifest binds fixes and runs")
        check(lr.manifest_digest({**doc, "fixes": ["other"]}) != digest, "digest covers the whole manifest")

        # Idempotent re-prepare: same content, same digest, no history churn.
        hist_before = len(lr.load_state(state)["history"])
        check(prepare(root, binary, evidence, commit, state) == 0, "re-prepare (same content) succeeds")
        check(digest_of(state) == digest, "same content re-prepare keeps the same digest")
        check(len(lr.load_state(state)["history"]) == hist_before, "idempotent re-prepare adds no event")

        # A new version keeps the old events instead of wiping history.
        check(prepare(root, binary, evidence, commit, state, fix="a new fix") == 0, "prepare a second version")
        st = lr.load_state(state)
        check(len(st["history"]) > hist_before and
              st["history"][-1]["event"] == "prepared" and
              st["history"][-1]["previous_digest"] == digest,
              "a new manifest appends a prepared event and keeps prior history")

        # --- failing counterexamples for the review findings ---
        def expect_unproven(text, why, maker="maker-1"):
            path = os.path.join(root, "bad-evidence-%s.md" % why)
            write(path, text)
            st_dir = os.path.join(root, "state-" + why)
            rc = lr.main(["prepare", "--state-dir", st_dir, "--feature", "f", "--version", "v",
                          "--source-commit", commit, "--binary", binary, "--evidence", path,
                          "--maker", maker])
            check(rc == lr.EXIT_EVIDENCE_UNPROVEN, "refused: " + why)

        # verdict FAIL but the word PASS appears elsewhere: must refuse.
        expect_unproven("# report\n- verdict: FAIL\n- checker: qa-bot\n- candidate: %s\n"
                        "- binary: %s\nAll other suites PASS.\n" % (commit, sha),
                        "verdict FAIL with PASS in prose")
        # empty checker.
        expect_unproven("# report\n- verdict: PASS\n- checker:   \n- candidate: %s\n- binary: %s\n"
                        % (commit, sha), "empty checker")
        # checker == maker.
        expect_unproven(evidence_text("maker-1", commit, sha), "self-checker")
        # missing binary sha.
        expect_unproven("# report\n- verdict: PASS\n- checker: qa-bot\n- candidate: %s\n" % commit,
                        "evidence without binary sha")
        # missing source commit.
        expect_unproven("# report\n- verdict: PASS\n- checker: qa-bot\n- binary: %s\n" % sha,
                        "evidence without source commit")

        check(lr.main(["approve", "--state-dir", state, "--manifest-digest", "0" * 64,
                       "--actor", "lai"]) == lr.EXIT_STALE, "a signature with the wrong digest is refused")
        check(approve(state) == 0, "a matching signature is accepted")
        make_binary(binary, BROKEN_BINARY)
        tampered = os.path.join(root, "state-tampered")
        shutil.copytree(state, tampered)
        check(lr.main(["approve", "--state-dir", tampered, "--manifest-digest", digest_of(tampered),
                       "--actor", "lai"]) == lr.EXIT_MANIFEST_CHANGED,
              "a changed binary refuses a new approval")
        target = make_target(root, label="old")
        rc, out = run_install(tampered, target)
        check(rc == lr.EXIT_MANIFEST_CHANGED, "a changed binary refuses install: %s" % out[-200:])
    finally:
        shutil.rmtree(root, ignore_errors=True)


def suite_install():
    root = tempfile.mkdtemp(prefix="lr-install-")
    env = isolated_env(root)
    try:
        binary, evidence, commit, sha, state = setup_case(root)
        check(prepare(root, binary, evidence, commit, state) == 0, "prepare ok")
        check(approve(state) == 0, "approve ok")
        target = make_target(root, label="old-binary")
        old_sha = lr.sha256_file(target)
        sibling = os.path.join(os.path.dirname(target), "keep-me.txt")
        write(sibling, "a work file that is not ours\n")

        # Preimage is mandatory for an existing target.
        rc, out = run_install(state, target, env=env)
        check(rc == lr.EXIT_STALE, "install without --expect-old-sha on an existing target is refused")
        check(lr.sha256_file(target) == old_sha, "the refused no-preimage call left the target untouched")

        rc, out = run_install(state, target, expect_old=old_sha, env=env)
        check(rc == 0, "install succeeds with the expected preimage: %s" % out[-200:])
        check(lr.sha256_file(target) == sha, "target holds the verified binary")
        check(os.path.exists(sibling), "installation did not delete neighbouring work files")
        st = lr.load_state(state)
        check(st["status"] == "installed" and st["installed"]["sha256"] == sha, "installed recorded with the sha")
        check(lr.sha256_file(st["history"][-1]["backup"]) == old_sha, "the old binary was backed up verifiably")

        rc, out = run_install(state, target, expect_old=old_sha, env=env)
        check(rc == lr.EXIT_STALE, "a stale expected-old preimage is refused")
        check(lr.sha256_file(target) == sha, "the refused stale call left the target untouched")

        # Two installers with DIFFERENT state dirs, same target: the
        # target lock (not the state dir) must serialise them.
        root2 = tempfile.mkdtemp(prefix="lr-race-")
        try:
            env2 = isolated_env(root2)
            b2, e2, c2, s2a, s2b = setup_race(root2)
            old2 = lr.sha256_file(make_target(root2, label="old-race"))
            t2 = os.path.join(root2, "opt", "beacon")
            argv = lambda s: [sys.executable, SCRIPT, "install", "--state-dir", s,
                              "--target", t2, "--expect-old-sha", old2]
            p1 = subprocess.Popen(argv(s2a), stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                                  text=True, env=env2)
            p2 = subprocess.Popen(argv(s2b), stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                                  text=True, env=env2)
            p1.communicate(timeout=60)
            p2.communicate(timeout=60)
            codes = sorted([p1.returncode, p2.returncode])
            check(codes == [0, lr.EXIT_STALE],
                  "different state dirs, same target: exactly one installs (%r)" % (codes,))
        finally:
            shutil.rmtree(root2, ignore_errors=True)

        # Crash between replace and record.
        root3 = tempfile.mkdtemp(prefix="lr-crash-")
        try:
            env3 = isolated_env(root3)
            b3, e3, c3, sha3, s3 = setup_case(root3)
            check(prepare(root3, b3, e3, c3, s3) == 0, "prepare ok (crash case)")
            check(approve(s3) == 0, "approve ok (crash case)")
            signed_at = lr.load_state(s3)["approval"]["at"]
            t3 = make_target(root3, label="old-crash")
            old3 = lr.sha256_file(t3)
            crash_env = dict(env3, BEACON_RELEASE_CRASH_AFTER_REPLACE="1")
            rc, out = run_install(s3, t3, expect_old=old3, env=crash_env)
            check(rc == 70, "simulated crash returns 70: %s" % out[-120:])
            st = lr.load_state(s3)
            check(st["status"] == "installing" and st["intent"] is not None, "crash left a persisted intent")
            check(lr.sha256_file(t3) == sha3, "the crash happened after the atomic replace")

            # Resume WITH the original preimage: this used to be refused.
            rc, out = run_install(s3, t3, expect_old=old3, env=env3)
            check(rc == 0, "resume with the original preimage succeeds: %s" % out[-200:])
            st = lr.load_state(s3)
            check(st["status"] == "installed" and st["approval"]["at"] == signed_at,
                  "resume installs without a second approval")

            # Resume WITHOUT a preimage must also finish (target is the
            # candidate), not overwrite unknown content.
            root4 = tempfile.mkdtemp(prefix="lr-crash2-")
            try:
                env4 = isolated_env(root4)
                b4, e4, c4, sha4, s4 = setup_case(root4)
                check(prepare(root4, b4, e4, c4, s4) == 0, "prepare ok (resume-no-preimage)")
                check(approve(s4) == 0, "approve ok (resume-no-preimage)")
                t4 = make_target(root4, label="old-crash2")
                old4 = lr.sha256_file(t4)
                rc, out = run_install(s4, t4, expect_old=old4,
                                      env=dict(env4, BEACON_RELEASE_CRASH_AFTER_REPLACE="1"))
                check(rc == 70, "crash (resume-no-preimage)")
                rc, out = run_install(s4, t4, env=env4)
                check(rc == 0, "resume without a preimage finishes: %s" % out[-160:])
                # Tampered unknown target: refuse and preserve.
                some_unknown = os.path.join(root4, "opt", "beacon")
                make_binary(some_unknown, "#!/bin/sh\necho unknown\n")
                unknown_sha = lr.sha256_file(some_unknown)
                st = lr.load_state(s4)
                st["intent"] = {"manifest_digest": digest_of(s4), "target": t4,
                                "old_sha256": old4, "candidate_sha256": sha4,
                                "backup": None, "at": "2026-01-01T00:00:00Z"}
                st["status"] = "installing"
                lr.save_state(s4, st)
                rc, out = run_install(s4, t4, env=env4)
                check(rc == lr.EXIT_TARGET_DRIFT, "resume over unknown content is refused (drift)")
                check(lr.sha256_file(t4) == unknown_sha, "unknown content was preserved")

                # Resume after the bound evidence changed: refuse.
                st = lr.load_state(s4)
                st["intent"] = {"manifest_digest": digest_of(s4), "target": t4,
                                "old_sha256": unknown_sha, "candidate_sha256": sha4,
                                "backup": None, "at": "2026-01-01T00:00:00Z"}
                st["status"] = "installing"
                lr.save_state(s4, st)
                write(e4, evidence_text("qa-bot", c4, sha4) + "\nappendix\n")
                rc, out = run_install(s4, t4, env=env4)
                check(rc == lr.EXIT_MANIFEST_CHANGED, "resume after evidence change is refused")
            finally:
                shutil.rmtree(root4, ignore_errors=True)

            # A binary that only answers --version fails the functional
            # probe and is never recorded as installed.
            root5 = tempfile.mkdtemp(prefix="lr-broken-")
            try:
                env5 = isolated_env(root5)
                bad = os.path.join(root5, "dist", "beacon")
                make_binary(bad, BROKEN_BINARY)
                bad_sha = lr.sha256_file(bad)
                ev = os.path.join(root5, "qa.md")
                write(ev, evidence_text("qa-bot", c3, bad_sha))
                s5 = os.path.join(root5, "state")
                check(lr.main(["prepare", "--state-dir", s5, "--feature", "f", "--version", "v",
                               "--source-commit", c3, "--binary", bad, "--evidence", ev,
                               "--maker", "maker-1"]) == 0, "prepare a version-only binary")
                check(approve(s5) == 0, "approve the version-only binary")
                t5 = make_target(root5, label="old")
                rc, out = run_install(s5, t5, expect_old=lr.sha256_file(t5), env=env5)
                check(rc == 1, "install refuses a binary that fails the tree probe")
                st = lr.load_state(s5)
                check(st["status"] == "failed" and st["failure"], "failed install records a reason")
                check(st["failure"].get("backup") and os.path.exists(st["failure"]["backup"]),
                      "failed install keeps a backup")
            finally:
                shutil.rmtree(root5, ignore_errors=True)
        finally:
            shutil.rmtree(root3, ignore_errors=True)
    finally:
        shutil.rmtree(root, ignore_errors=True)


def setup_race(root):
    """Two independent state dirs for the SAME candidate; one target."""
    binary = os.path.join(root, "dist", "beacon")
    make_binary(binary, GOOD_BINARY)
    sha = lr.sha256_file(binary)
    commit = "b" * 40
    evidence = os.path.join(root, "qa-report.md")
    write(evidence, evidence_text("qa-bot", commit, sha))
    s1 = os.path.join(root, "state-a")
    s2 = os.path.join(root, "state-b")
    for st_dir in (s1, s2):
        assert prepare(root, binary, evidence, commit, st_dir) == 0
        assert approve(st_dir) == 0
    return binary, evidence, commit, s1, s2


def suite_status():
    root = tempfile.mkdtemp(prefix="lr-status-")
    env = isolated_env(root)
    try:
        binary, evidence, commit, sha, state = setup_case(root)
        check(prepare(root, binary, evidence, commit, state) == 0, "prepare ok")
        check(approve(state) == 0, "approve ok")
        target = make_target(root, label="old")
        rc, out = run_install(state, target, expect_old=lr.sha256_file(target), env=env)
        check(rc == 0, "install ok: %s" % out[-160:])
        installed_history = len(lr.load_state(state)["history"])
        check(lr.main(["status", "--state-dir", state, "--target", target]) == 0,
              "status aligned (hash + tree probe) -> 0")

        make_binary(target, BROKEN_BINARY)
        rc = lr.main(["status", "--state-dir", state, "--target", target])
        check(rc == lr.EXIT_DRIFTED, "status drifted -> 60")
        st = lr.load_state(state)
        check(len(st["history"]) == installed_history and st["status"] == "installed",
              "drift observation keeps history and does not auto-repair")

        os.remove(target)
        check(lr.main(["status", "--state-dir", state, "--target", target]) == lr.EXIT_MISSING,
              "status missing -> 61")

        shutil.copy2(binary, target)
        os.chmod(target, 0o755)
        check(lr.main(["status", "--state-dir", state, "--target", target]) == 0, "aligned again -> 0")

        # A hash match alone is not delivery: a legacy install of a
        # version-only binary must report not_usable, not aligned.
        root2 = tempfile.mkdtemp(prefix="lr-status-legacy-")
        try:
            bad = os.path.join(root2, "dist", "beacon")
            make_binary(bad, BROKEN_BINARY)
            bad_sha = lr.sha256_file(bad)
            ev = os.path.join(root2, "qa.md")
            write(ev, evidence_text("qa-bot", commit, bad_sha))
            s2 = os.path.join(root2, "state")
            lr.main(["prepare", "--state-dir", s2, "--feature", "f", "--version", "v",
                     "--source-commit", commit, "--binary", bad, "--evidence", ev,
                     "--maker", "maker-1"])
            st = lr.load_state(s2)
            st["installed"] = {"manifest_digest": digest_of(s2), "target": os.path.join(root2, "opt", "beacon"),
                               "sha256": bad_sha, "at": "2026-01-01T00:00:00Z"}
            st["status"] = "installed"
            lr.save_state(s2, st)
            t2 = make_target(root2, body=BROKEN_BINARY)
            check(lr.sha256_file(t2) == bad_sha, "legacy target hash matches the installed record")
            rc = lr.main(["status", "--state-dir", s2, "--target", t2])
            check(rc == lr.EXIT_NOT_USABLE, "hash match without the tree probe -> 68, not aligned")
        finally:
            shutil.rmtree(root2, ignore_errors=True)

        empty = os.path.join(root, "empty-state")
        os.makedirs(empty, exist_ok=True)
        check(lr.main(["status", "--state-dir", empty, "--target", target]) == lr.EXIT_UNKNOWN_MANIFEST,
              "status unknown manifest -> 62")
    finally:
        shutil.rmtree(root, ignore_errors=True)


SUITES = {"manifest": suite_manifest, "install": suite_install, "status": suite_status}


def main():
    suite = sys.argv[1] if len(sys.argv) > 1 else ""
    if suite not in SUITES:
        print("usage: test_local_release.py {manifest|install|status}", file=sys.stderr)
        return 2
    try:
        SUITES[suite]()
    except AssertionError as exc:
        print("FAIL [%s]: %s" % (suite, exc), file=sys.stderr)
        return 1
    except Exception as exc:  # noqa: BLE001
        print("ERROR [%s]: %r" % (suite, exc), file=sys.stderr)
        return 1
    for line in PASS:
        print("ok -", line)
    print("PASS [%s]: %d checks" % (suite, len(PASS)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
