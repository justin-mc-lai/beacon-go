#!/usr/bin/env python3
"""Behavioural tests for scripts/local_release.py.

Run one suite at a time (the commands are the acceptance contract):
    python3 scripts/test_local_release.py manifest
    python3 scripts/test_local_release.py install
    python3 scripts/test_local_release.py status

No framework: plain asserts, a real temp filesystem per case, real
subprocesses for the concurrent-installer and crash-resume cases.
"""
import hashlib
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


def make_fake_binary(path, label):
    write(path, "#!/bin/sh\necho 'Beacon CLI %s'\nexit 0\n" % label, 0o755)


def evidence_text(checker, commit, verdict="PASS"):
    return ("# Independent QA report\n\n"
            "- verdict: %s\n"
            "- candidate: %s\n"
            "- checker: %s\n"
            "- tests: go test ./... -count=1 (exit 0)\n" % (verdict, commit, checker))


def setup_case(root):
    bindir = os.path.join(root, "dist")
    binary = os.path.join(bindir, "beacon")
    make_fake_binary(binary, "v9-candidate")
    evidence = os.path.join(root, "qa-report.md")
    commit = "a" * 40
    write(evidence, evidence_text("qa-bot", commit))
    state = os.path.join(root, "state")
    return binary, evidence, commit, state


def prepare(root, binary, evidence, commit, state, maker="maker-1"):
    return lr.main(["prepare", "--state-dir", state, "--feature", "kernel-local-release-closure",
                    "--version", "v2.0.0-go.15", "--requirements", "fp-req-1",
                    "--source-commit", commit, "--fix", "integrity path fix",
                    "--build-cmd", "go build -trimpath -o dist/beacon ./cmd/beacon",
                    "--build-conditions", "darwin/arm64", "--binary", binary,
                    "--evidence", evidence, "--run-id", "run_1", "--maker", maker])


def digest_of(state):
    with open(os.path.join(state, "manifest.json"), encoding="utf-8") as f:
        return json.load(f)["digest"]


def run_install(state, target, expect_old=None, env=None):
    argv = [sys.executable, SCRIPT, "install", "--state-dir", state, "--target", target]
    if expect_old is not None:
        argv += ["--expect-old-sha", expect_old]
    proc = subprocess.run(argv, capture_output=True, text=True, env=env)
    return proc.returncode, proc.stdout + proc.stderr


def make_target(root, label="old"):
    target = os.path.join(root, "opt", "beacon")
    make_fake_binary(target, label)
    return target


# ------------------------------------------------------------------


def suite_manifest():
    with tempfile.TemporaryDirectory() as root:
        binary, evidence, commit, state = setup_case(root)
        rc = prepare(root, binary, evidence, commit, state)
        check(rc == 0, "prepare binds a valid candidate")
        digest = digest_of(state)
        check(len(digest) == 64, "manifest has a 64-char digest")

        with open(os.path.join(state, "manifest.json"), encoding="utf-8") as f:
            doc = json.load(f)
        check(doc["source_commit"] == commit and doc["binary"]["sha256"] == lr.sha256_file(binary),
              "manifest binds source commit and binary sha")
        check(len(doc["evidence"]) == 1 and doc["evidence"][0]["sha256"] == lr.sha256_file(evidence),
              "manifest binds evidence hash")
        check(doc["fixes"] == ["integrity path fix"] and doc["build"]["conditions"] == "darwin/arm64",
              "manifest binds fixes and build conditions")
        check(doc["runs"] == ["run_1"], "manifest binds the run list")

        # Every field is inside the digest: changing a fix changes it.
        altered = dict(doc); altered["fixes"] = ["something else"]
        check(lr.manifest_digest(altered) != digest, "digest covers the whole manifest")

        # A wrong digest cannot approve.
        rc = lr.main(["approve", "--state-dir", state, "--manifest-digest", "0" * 64, "--actor", "lai"])
        check(rc == lr.EXIT_STALE, "a signature with the wrong digest is refused")

        # Evidence that does not prove independent QA cannot prepare.
        bad = os.path.join(root, "bad-evidence.md")
        write(bad, "# notes\n\nlooks fine to me\n")
        bad_state = os.path.join(root, "state-bad")
        rc = lr.main(["prepare", "--state-dir", bad_state, "--feature", "f", "--version", "v",
                      "--source-commit", commit, "--binary", binary, "--evidence", bad, "--maker", "maker-1"])
        check(rc == lr.EXIT_EVIDENCE_UNPROVEN, "evidence without a verdict/checker is refused")

        selfsigned = os.path.join(root, "self-evidence.md")
        write(selfsigned, evidence_text("maker-1", commit))
        sstate = os.path.join(root, "state-self")
        rc = lr.main(["prepare", "--state-dir", sstate, "--feature", "f", "--version", "v",
                      "--source-commit", commit, "--binary", binary, "--evidence", selfsigned,
                      "--maker", "maker-1"])
        check(rc == lr.EXIT_EVIDENCE_UNPROVEN, "the maker cannot be its own QA checker")

        # Approving after the binary changed on disk is refused.
        rc = lr.main(["approve", "--state-dir", state, "--manifest-digest", digest, "--actor", "lai"])
        check(rc == 0, "a matching signature is accepted")
        make_fake_binary(binary, "tampered")
        rstate = os.path.join(root, "state-tamper")
        shutil.copytree(state, rstate)
        rc = lr.main(["approve", "--state-dir", rstate, "--manifest-digest", digest, "--actor", "lai"])
        check(rc == lr.EXIT_MANIFEST_CHANGED, "a changed binary refuses a new approval")

        # And it refuses install.
        target = make_target(root)
        rc, out = run_install(rstate, target)
        check(rc == lr.EXIT_MANIFEST_CHANGED, "a changed binary refuses install: %s" % out[-200:])


def suite_install():
    with tempfile.TemporaryDirectory() as root:
        binary, evidence, commit, state = setup_case(root)
        check(prepare(root, binary, evidence, commit, state) == 0, "prepare ok")
        digest = digest_of(state)
        check(lr.main(["approve", "--state-dir", state, "--manifest-digest", digest, "--actor", "lai"]) == 0,
              "approve ok")
        target = make_target(root, "old-binary")
        old_sha = lr.sha256_file(target)
        sibling = os.path.join(os.path.dirname(target), "keep-me.txt")
        write(sibling, "a work file that is not ours\n")

        rc, out = run_install(state, target, expect_old=old_sha)
        check(rc == 0, "install succeeds with the expected preimage: %s" % out[-200:])
        check(lr.sha256_file(target) == lr.sha256_file(binary), "target now holds the verified binary")
        check(os.path.exists(sibling), "installation did not delete neighbouring work files")
        st = lr.load_state(state)
        check(st["status"] == "installed" and st["installed"]["sha256"] == lr.sha256_file(binary),
              "install recorded installed with the binary sha")
        backup = st["history"][-1]["backup"]
        check(backup and lr.sha256_file(backup) == old_sha, "the old binary was backed up verifiably")

        # Stale preimage: a second call that still expects the OLD hash
        # must refuse, not overwrite the newly installed binary.
        rc, out = run_install(state, target, expect_old=old_sha)
        check(rc == lr.EXIT_STALE, "a stale expected-old preimage is refused")
        check(lr.sha256_file(target) == lr.sha256_file(binary), "the refused call left the target untouched")

        # Two simultaneous installers: serialised, exactly one wins.
        with tempfile.TemporaryDirectory() as root2:
            b2, e2, c2, s2 = setup_case(root2)
            check(prepare(root2, b2, e2, c2, s2) == 0, "prepare ok (race case)")
            d2 = digest_of(s2)
            check(lr.main(["approve", "--state-dir", s2, "--manifest-digest", d2, "--actor", "lai"]) == 0,
                  "approve ok (race case)")
            t2 = make_target(root2, "old-race")
            old2 = lr.sha256_file(t2)
            argv = [sys.executable, SCRIPT, "install", "--state-dir", s2, "--target", t2,
                    "--expect-old-sha", old2]
            p1 = subprocess.Popen(argv, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
            p2 = subprocess.Popen(argv, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
            o1, _ = p1.communicate(timeout=60)
            o2, _ = p2.communicate(timeout=60)
            codes = sorted([p1.returncode, p2.returncode])
            check(codes == [0, lr.EXIT_STALE],
                  "two concurrent installers: one installs, one refuses stale (%r)" % (codes,))

        # Crash between replace and record: resume installs without a
        # second approval.
        with tempfile.TemporaryDirectory() as root3:
            b3, e3, c3, s3 = setup_case(root3)
            check(prepare(root3, b3, e3, c3, s3) == 0, "prepare ok (crash case)")
            d3 = digest_of(s3)
            check(lr.main(["approve", "--state-dir", s3, "--manifest-digest", d3, "--actor", "lai"]) == 0,
                  "approve ok (crash case)")
            st_before = lr.load_state(s3)
            signed_at = st_before["approval"]["at"]
            t3 = make_target(root3, "old-crash")
            env = dict(os.environ, BEACON_RELEASE_CRASH_AFTER_REPLACE="1")
            rc, out = run_install(s3, t3, env=env)
            check(rc == 70, "simulated crash returns 70: %s" % out[-120:])
            st = lr.load_state(s3)
            check(st["status"] == "installing" and st["intent"] is not None,
                  "crash left a persisted intent (not installed yet)")
            check(lr.sha256_file(t3) == lr.sha256_file(b3), "the crash happened after the atomic replace")
            rc, out = run_install(s3, t3)
            check(rc == 0, "resume installs from the intent: %s" % out[-200:])
            st = lr.load_state(s3)
            check(st["status"] == "installed", "resume recorded installed")
            check(st["approval"]["at"] == signed_at, "resume did not require a second approval")


def suite_status():
    with tempfile.TemporaryDirectory() as root:
        binary, evidence, commit, state = setup_case(root)
        check(prepare(root, binary, evidence, commit, state) == 0, "prepare ok")
        digest = digest_of(state)
        check(lr.main(["approve", "--state-dir", state, "--manifest-digest", digest, "--actor", "lai"]) == 0,
              "approve ok")
        target = make_target(root, "old")
        rc, out = run_install(state, target, expect_old=lr.sha256_file(target))
        check(rc == 0, "install ok: %s" % out[-160:])
        st = lr.load_state(state)
        installed_history = len(st["history"])

        check(lr.main(["status", "--state-dir", state, "--target", target]) == 0, "status aligned -> 0")

        # External overwrite: drifted, non-zero, history preserved.
        make_fake_binary(target, "someone-else")
        rc = lr.main(["status", "--state-dir", state, "--target", target])
        check(rc == lr.EXIT_DRIFTED, "status drifted -> 60")
        st = lr.load_state(state)
        check(len(st["history"]) == installed_history and st["status"] == "installed",
              "drift observation keeps the installed history and does not auto-repair")
        check(lr.sha256_file(target) != lr.sha256_file(binary), "status did not overwrite the unknown binary")

        # Missing: non-zero, history still there.
        os.remove(target)
        rc = lr.main(["status", "--state-dir", state, "--target", target])
        check(rc == lr.EXIT_MISSING, "status missing -> 61")
        st = lr.load_state(state)
        check(len(st["history"]) == installed_history, "missing observation keeps history")

        # Back to the installed binary: aligned again.
        shutil.copy2(binary, target)
        os.chmod(target, 0o755)
        check(lr.main(["status", "--state-dir", state, "--target", target]) == 0, "status aligned again -> 0")

        # Unknown manifest: deterministic code, no repair.
        empty = os.path.join(root, "empty-state")
        os.makedirs(empty, exist_ok=True)
        rc = lr.main(["status", "--state-dir", empty, "--target", target])
        check(rc == lr.EXIT_UNKNOWN_MANIFEST, "status unknown manifest -> 62")


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
