#!/usr/bin/env python3
"""One-click negative controls for the read-only kernel maintainer.

Proves, without running the production script or touching the live binary:
  * aligned / drifted / missing / unknown-manifest map to the same exit
    codes as local_release.py status;
  * with fake go / beacon / beacon-go / launchctl / git first on PATH and
    a target sentinel, a maintainer run fires no sentinel, rewrites no
    target, creates no build artifact and changes no release state;
  * the maintenance-body path copies text verbatim: a body containing an
    unescaped backtick and a dollar-paren command does NOT execute it;
  * the script and the launchd template contain no backtick and no
    command substitution, and the template argv is one absolute path.

Usage: python3 scripts/test_kernel_maintainer.py
Exit 0 on all checks, 1 on failure.
"""
import hashlib
import json
import os
import plistlib
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(HERE)
SCRIPT = os.path.join(HERE, "kernel_maintainer_readonly.sh")
PLIST = os.path.join(HERE, "launchd", "icu.omb.kernel.plist")
RELEASE = os.path.join(HERE, "local_release.py")
DEFAULT_BIN = os.path.join(REPO, ".beacon", "evidence",
                           "kernel-local-release-closure", "bin", "beacon")
BACKTICK = chr(96)
DOLLAR_PAREN = "$" + "("
PASS = []


def check(cond, msg):
    if not cond:
        raise AssertionError(msg)
    PASS.append(msg)


def sha(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def write(path, text, mode=None):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(text)
    if mode:
        os.chmod(path, mode)


def run(argv, env):
    return subprocess.run(argv, capture_output=True, text=True, env=env)


def build_state(root, binary_src):
    state = os.path.join(root, "state")
    target = os.path.join(root, "target", "beacon")
    os.makedirs(os.path.dirname(target), exist_ok=True)
    src = os.path.join(root, "dist", "beacon")
    os.makedirs(os.path.dirname(src), exist_ok=True)
    shutil.copy2(binary_src, src)
    os.chmod(src, 0o755)
    commit = "c" * 40
    evidence = os.path.join(root, "qa.md")
    write(evidence, "# QA\n- verdict: PASS\n- candidate: %s\n- binary: %s\n- checker: qa-bot\n"
          % (commit, sha(src)))
    check(run([sys.executable, RELEASE, "prepare", "--state-dir", state,
               "--feature", "kernel-local-release-closure", "--version", "v2.0.0-go.15",
               "--source-commit", commit, "--binary", src, "--evidence", evidence,
               "--maker", "maker-1"], dict(os.environ)).returncode == 0,
          "temp manifest prepared")
    with open(os.path.join(state, "manifest.json"), encoding="utf-8") as f:
        digest = json.load(f)["digest"]
    check(run([sys.executable, RELEASE, "approve", "--state-dir", state,
               "--manifest-digest", digest, "--actor", "lai"], dict(os.environ)).returncode == 0,
          "temp manifest approved")
    inst = run([sys.executable, RELEASE, "install", "--state-dir", state,
                "--target", target], dict(os.environ))
    check(inst.returncode == 0, "temp target installed: " + (inst.stdout + inst.stderr)[-200:])
    return state, target


def main():
    binary_src = os.environ.get("CANDIDATE_BIN", DEFAULT_BIN)
    if not os.path.isfile(binary_src):
        print("missing candidate binary: " + binary_src, file=sys.stderr)
        return 1
    root = tempfile.mkdtemp(prefix="maintain-test-")
    try:
        state, target = build_state(root, binary_src)
        target_sha = sha(target)

        # Sentinels: any maintenance command on PATH leaves a marker.
        sentinel = os.path.join(root, "sentinel")
        os.makedirs(sentinel)
        fake_bin = os.path.join(root, "fakebin")
        os.makedirs(fake_bin)
        for name in ("go", "beacon", "beacon-go", "launchctl", "git", "om"):
            write(os.path.join(fake_bin, name),
                  "#!/bin/sh\ntouch " + sentinel + "/" + name + "\n", 0o755)

        drift_log = os.path.join(root, "release-drift.log")
        env = dict(os.environ)
        env.update({
            "BEACON_REPO": REPO,
            "BEACON_RELEASE_STATE": state,
            "BEACON_TARGET": target,
            "BEACON_DRIFT_LOG": drift_log,
            "PATH": fake_bin + ":/usr/bin:/bin:/usr/sbin:/sbin",
        })
        state_files_before = sorted(os.listdir(state))
        state_sha_before = sha(os.path.join(state, "state.json"))
        target_mtime_before = os.stat(target).st_mtime_ns
        sentinels_before = sorted(os.listdir(sentinel))

        # 1) Bent body: an old-style quoted assignment WOULD execute. This
        #    is the defect class, kept only as a positive control.
        marker = os.path.join(root, "old-marker")
        old_script = 'BODY="body: ' + BACKTICK + 'touch ' + marker + BACKTICK + '"\n'
        old = subprocess.run(["/bin/sh", "-c", old_script],
                            capture_output=True, text=True, env=env)
        check(old.returncode == 0 and os.path.exists(marker),
              "positive control: a double-quoted backtick body executes")

        # 2) Aligned: exit 0, nothing executed or written.
        obs = run([SCRIPT], env)
        check(obs.returncode == 0,
              "aligned -> 0: " + (obs.stdout + obs.stderr)[-200:])
        check(sha(target) == target_sha and os.stat(target).st_mtime_ns == target_mtime_before,
              "aligned run left the target bytes and mtime untouched")
        check(sha(os.path.join(state, "state.json")) == state_sha_before
              and sorted(os.listdir(state)) == state_files_before,
              "aligned run left release state untouched")
        check(not os.path.exists(drift_log), "aligned run wrote no drift log")
        check(sorted(os.listdir(sentinel)) == sentinels_before,
              "aligned run executed no fake go/beacon/launchctl/git")

        # 3) Drifted: same code as status, target untouched, one log record.
        with open(target, "ab") as f:
            f.write(b"\n# external tamper\n")
        drifted_sha = sha(target)
        obs2 = run([SCRIPT], env)
        check(obs2.returncode == 60,
              "drifted -> 60 (got %d)" % obs2.returncode)
        check(sha(target) == drifted_sha, "drift run did not touch the unknown target")
        with open(drift_log, encoding="utf-8") as f:
            lines = [ln for ln in f.read().splitlines() if ln]
        check(any("drift exit=60" in ln for ln in lines), "drift log records exit 60")
        check(sorted(os.listdir(sentinel)) == sentinels_before,
              "drift run executed no maintenance command")

        # 4) Missing: 61.
        os.remove(target)
        check(run([SCRIPT], env).returncode == 61, "missing -> 61")

        # 5) Unknown manifest: 62.
        empty = os.path.join(root, "empty-state")
        os.makedirs(empty, exist_ok=True)
        env_empty = dict(env, BEACON_RELEASE_STATE=empty)
        check(run([SCRIPT], env_empty).returncode == 62, "unknown manifest -> 62")

        # 6) Body path is data: backtick and dollar-paren stay literal.
        body = ("maintenance body\n"
                "1. run " + BACKTICK + "touch " + sentinel + "/body" + BACKTICK + "\n"
                "2. run " + DOLLAR_PAREN + "touch " + sentinel + "/body2)\n")
        body_file = os.path.join(root, "body.md")
        write(body_file, body)
        rendered = run([SCRIPT, "render-body", body_file], env)
        check(rendered.returncode == 0, "render-body exits 0")
        check(rendered.stdout == body, "render-body emits the body byte-for-byte")
        check(sorted(os.listdir(sentinel)) == sentinels_before,
              "body text was not evaluated (no body/body2 sentinel)")

        # 7) Static: no evaluated syntax in the script or the template.
        for path in (SCRIPT, PLIST):
            with open(path, encoding="utf-8") as f:
                text = f.read()
            name = os.path.basename(path)
            check(BACKTICK not in text, name + " contains no backtick")
            check(DOLLAR_PAREN not in text, name + " contains no command substitution")
        with open(PLIST, "rb") as f:
            plist = plistlib.load(f)
        argv = plist["ProgramArguments"]
        check(argv == ["/Users/apple/Developer/Personal/products/beacon-go/scripts/"
                       "kernel_maintainer_readonly.sh"],
              "template argv is one absolute repository script path")
        check(argv[0].endswith("kernel_maintainer_readonly.sh"), "template points at the read-only script")
        check(plist["StartInterval"] == 3600, "template StartInterval is 3600")
        check(plist.get("RunAtLoad") is True, "template RunAtLoad is true")
        check(all(a.startswith("/") for a in argv), "all ProgramArguments are absolute")

        for line in PASS:
            print("ok -", line)
        print("PASS [maintainer]: %d checks" % len(PASS))
        return 0
    except AssertionError as exc:
        print("FAIL [maintainer]: %s" % exc, file=sys.stderr)
        return 1
    finally:
        shutil.rmtree(root, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())
