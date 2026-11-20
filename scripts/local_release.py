#!/usr/bin/env python3
"""Local release closure for Beacon Go (kernel-local-release-closure
AC-001..AC-003). Standard library only.

One candidate -> one manifest digest -> one human approval -> one
atomic install -> an install history that survives drift.

Subcommands
-----------
prepare   Bind requirements fingerprint, source commit, fixes, build
          conditions, the built binary's SHA-256, the independent QA
          evidence hashes and the run list into ONE manifest digest.
approve   Persist a human signature that matches that whole digest.
install   Lock the target, check the expected-old preimage, persist
          intent + backup, atomically replace, verify hash + function.
status    Compare the recorded install against the real target:
          aligned / drifted / missing, with a stable non-zero code.

Exit codes
----------
 0  ok / aligned
 1  operational failure
 60 drifted            61 missing
 62 unknown manifest   63 not installed
 64 stale target preimage / not approved
 65 manifest no longer matches disk (binary or evidence changed)
 66 evidence does not prove an independent QA verdict
"""
import argparse
import fcntl
import hashlib
import json
import os
import shutil
import subprocess
import sys
import time

SCHEMA = "beacon-go/local-release-manifest.v1"
EXIT_DRIFTED, EXIT_MISSING = 60, 61
EXIT_UNKNOWN_MANIFEST, EXIT_NOT_INSTALLED = 62, 63
EXIT_STALE, EXIT_MANIFEST_CHANGED, EXIT_EVIDENCE_UNPROVEN = 64, 65, 66


class ReleaseError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def canonical(doc):
    return json.dumps(doc, sort_keys=True, separators=(",", ":"), ensure_ascii=False)


def manifest_digest(doc):
    body = {k: v for k, v in doc.items() if k != "digest"}
    return hashlib.sha256(canonical(body).encode("utf-8")).hexdigest()


def atomic_write_json(path, doc):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    tmp = path + ".tmp"
    with open(tmp, "w", encoding="utf-8") as f:
        json.dump(doc, f, indent=2, sort_keys=True, ensure_ascii=False)
        f.flush()
        os.fsync(f.fileno())
    os.replace(tmp, path)


def read_json(path):
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def state_paths(state_dir):
    return (
        os.path.join(state_dir, "manifest.json"),
        os.path.join(state_dir, "state.json"),
    )


def load_state(state_dir):
    _, sp = state_paths(state_dir)
    if not os.path.exists(sp):
        return {"status": "prepared", "history": [], "approval": None,
                "intent": None, "installed": None, "failure": None}
    return read_json(sp)


def save_state(state_dir, st):
    _, sp = state_paths(state_dir)
    atomic_write_json(sp, st)


def load_manifest(state_dir):
    mp, _ = state_paths(state_dir)
    if not os.path.exists(mp):
        raise ReleaseError(EXIT_UNKNOWN_MANIFEST, "no release manifest at " + mp)
    doc = read_json(mp)
    if doc.get("schema") != SCHEMA:
        raise ReleaseError(EXIT_UNKNOWN_MANIFEST, "unknown manifest schema %r" % doc.get("schema"))
    if doc.get("digest") != manifest_digest(doc):
        raise ReleaseError(EXIT_MANIFEST_CHANGED, "manifest digest does not match its own body")
    return doc


def verify_evidence_file(path, source_commit, binary_sha, maker):
    """Evidence must state a verdict, name its checker (not the maker)
    and identify the candidate. A random file cannot self-certify QA."""
    with open(path, encoding="utf-8", errors="replace") as f:
        body = f.read().lower()
    if "verdict" not in body and "判定" not in body:
        raise ReleaseError(EXIT_EVIDENCE_UNPROVEN, "evidence %s has no verdict line" % path)
    if "pass" not in body and "通过" not in body:
        raise ReleaseError(EXIT_EVIDENCE_UNPROVEN, "evidence %s does not state a passing verdict" % path)
    if "checker" not in body and "检查者" not in body:
        raise ReleaseError(EXIT_EVIDENCE_UNPROVEN, "evidence %s names no checker" % path)
    if not (source_commit.lower() in body or binary_sha.lower() in body):
        raise ReleaseError(EXIT_EVIDENCE_UNPROVEN,
                           "evidence %s does not identify the candidate (source commit or binary sha)" % path)
    for token in ("checker", "检查者"):
        idx = body.find(token)
        if idx >= 0:
            rest = body[idx + len(token):].lstrip(" :：|\t")
            name = rest.split()[0] if rest.split() else ""
            name = name.strip("`*|")
            if maker and name == maker.lower():
                raise ReleaseError(EXIT_EVIDENCE_UNPROVEN,
                                   "the maker %r cannot be its own QA checker" % maker)
    return True


def cmd_prepare(args):
    if not os.path.isfile(args.binary):
        raise ReleaseError(1, "binary not found: " + args.binary)
    if not args.evidence:
        raise ReleaseError(EXIT_EVIDENCE_UNPROVEN, "at least one independent QA evidence file is required")
    binary_sha = sha256_file(args.binary)
    evidence = []
    for path in args.evidence:
        if not os.path.isfile(path):
            raise ReleaseError(1, "evidence not found: " + path)
        verify_evidence_file(path, args.source_commit, binary_sha, args.maker)
        evidence.append({"path": os.path.abspath(path), "sha256": sha256_file(path)})
    doc = {
        "schema": SCHEMA,
        "feature": args.feature,
        "version": args.version,
        "requirements": args.requirements,
        "source_commit": args.source_commit,
        "fixes": args.fix or [],
        "build": {"command": args.build_cmd, "conditions": args.build_conditions},
        "binary": {"path": os.path.abspath(args.binary), "sha256": binary_sha,
                   "size": os.path.getsize(args.binary)},
        "evidence": evidence,
        "runs": args.run_id or [],
        "maker": args.maker,
        "prepared_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
    }
    doc["digest"] = manifest_digest(doc)
    mp, _ = state_paths(args.state_dir)
    atomic_write_json(mp, doc)
    st = {"status": "prepared", "history": [], "approval": None, "intent": None,
          "installed": None, "failure": None}
    save_state(args.state_dir, st)
    return {"prepared": True, "manifest_digest": doc["digest"],
            "binary_sha256": binary_sha, "evidence": evidence}


def recheck_disk(doc):
    """The manifest is only valid while its binary and evidence still
    hash to what was bound. Any change refuses install."""
    if not os.path.isfile(doc["binary"]["path"]):
        raise ReleaseError(EXIT_MANIFEST_CHANGED, "bound binary is gone: " + doc["binary"]["path"])
    if sha256_file(doc["binary"]["path"]) != doc["binary"]["sha256"]:
        raise ReleaseError(EXIT_MANIFEST_CHANGED, "bound binary changed on disk")
    for ev in doc["evidence"]:
        if not os.path.isfile(ev["path"]):
            raise ReleaseError(EXIT_MANIFEST_CHANGED, "bound evidence is gone: " + ev["path"])
        if sha256_file(ev["path"]) != ev["sha256"]:
            raise ReleaseError(EXIT_MANIFEST_CHANGED, "bound evidence changed on disk: " + ev["path"])


def cmd_approve(args):
    doc = load_manifest(args.state_dir)
    st = load_state(args.state_dir)
    if args.manifest_digest != doc["digest"]:
        raise ReleaseError(EXIT_STALE, "signature digest does not match the prepared manifest")
    recheck_disk(doc)
    if st.get("status") not in ("prepared", "approved"):
        raise ReleaseError(EXIT_STALE, "release is %s; only a prepared release can be approved" % st.get("status"))
    st["approval"] = {"manifest_digest": doc["digest"], "actor": args.actor,
                      "at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())}
    st["status"] = "approved"
    st["history"].append({"event": "approved", "manifest_digest": doc["digest"], "actor": args.actor})
    save_state(args.state_dir, st)
    return {"approved": True, "manifest_digest": doc["digest"], "actor": args.actor}


def _lock(state_dir):
    os.makedirs(state_dir, exist_ok=True)
    f = open(os.path.join(state_dir, "install.lock"), "w")
    fcntl.flock(f.fileno(), fcntl.LOCK_EX)
    return f


def _backup(target, state_dir, stamp):
    os.makedirs(os.path.join(state_dir, "backups"), exist_ok=True)
    dst = os.path.join(state_dir, "backups", "target-%s" % stamp)
    if os.path.exists(target):
        shutil.copy2(target, dst)
        return dst
    return None


def _atomic_replace_source(source, target):
    tmp = target + ".new.%d" % os.getpid()
    shutil.copy2(source, tmp)
    os.chmod(tmp, 0o755)
    with open(tmp, "rb") as f:
        os.fsync(f.fileno())
    os.replace(tmp, target)


def _functional_check(target):
    try:
        proc = subprocess.run([target, "--version"], capture_output=True, text=True, timeout=20)
    except (OSError, subprocess.TimeoutExpired) as exc:
        return False, str(exc)
    if proc.returncode != 0:
        return False, (proc.stderr or proc.stdout or "").strip()
    return True, proc.stdout.strip()


def cmd_install(args):
    lock = _lock(args.state_dir)
    try:
        doc = load_manifest(args.state_dir)
        st = load_state(args.state_dir)
        target = os.path.abspath(args.target)
        os.makedirs(os.path.dirname(target), exist_ok=True)

        # The preimage is checked first: a stale second installer must
        # never overwrite a target another installer already replaced.
        current = sha256_file(target) if os.path.exists(target) else None
        if args.expect_old_sha is not None and current != args.expect_old_sha:
            raise ReleaseError(EXIT_STALE,
                               "expected old target sha256 %s but found %s; refusing stale overwrite"
                               % (args.expect_old_sha, current))

        intent = st.get("intent")
        resuming = bool(intent) and intent.get("manifest_digest") == doc["digest"]
        if resuming:
            # Crash after replace / before recording: same approval,
            # no second signature. Observe and finish.
            pass
        else:
            if st.get("status") not in ("approved", "installing") or not st.get("approval"):
                raise ReleaseError(EXIT_STALE, "release is not approved (status=%s)" % st.get("status"))
            if st["approval"].get("manifest_digest") != doc["digest"]:
                raise ReleaseError(EXIT_STALE, "approval does not match this manifest digest")
            recheck_disk(doc)

        if current == doc["binary"]["sha256"] and st.get("status") == "installed":
            return {"installed": True, "already": True, "target": target,
                    "sha256": current, "manifest_digest": doc["digest"]}

        if resuming and current == doc["binary"]["sha256"]:
            ok, detail = _functional_check(target)
            if not ok:
                _fail(st, args.state_dir, doc, target, intent.get("backup"), "functional check failed: " + detail)
                raise ReleaseError(1, "installed binary failed its functional check: " + detail)
            _record_installed(st, args.state_dir, doc, target, intent.get("backup"), resumed=True)
            return {"installed": True, "resumed": True, "target": target,
                    "sha256": current, "manifest_digest": doc["digest"],
                    "backup": intent.get("backup")}

        stamp = time.strftime("%Y%m%dT%H%M%S") + "-" + str(os.getpid())
        backup = _backup(target, args.state_dir, stamp)
        intent = {"manifest_digest": doc["digest"], "target": target, "old_sha256": current,
                  "backup": backup, "at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())}
        st["intent"] = intent
        st["status"] = "installing"
        if not resuming:
            st["history"].append({"event": "install_intent", "target": target,
                                  "old_sha256": current, "backup": backup})
        save_state(args.state_dir, st)

        _atomic_replace_source(doc["binary"]["path"], target)

        if os.environ.get("BEACON_RELEASE_CRASH_AFTER_REPLACE") == "1":
            # Test hook: simulate a crash between replace and record.
            os._exit(70)

        if sha256_file(target) != doc["binary"]["sha256"]:
            _fail(st, args.state_dir, doc, target, backup, "target hash differs after install")
            raise ReleaseError(1, "installed target hash does not match the manifest")
        ok, detail = _functional_check(target)
        if not ok:
            _fail(st, args.state_dir, doc, target, backup, "functional check failed: " + detail)
            raise ReleaseError(1, "installed binary failed its functional check: " + detail)
        _record_installed(st, args.state_dir, doc, target, backup, resumed=resuming)
        return {"installed": True, "target": target, "sha256": doc["binary"]["sha256"],
                "manifest_digest": doc["digest"], "backup": backup}
    finally:
        fcntl.flock(lock.fileno(), fcntl.LOCK_UN)
        lock.close()


def _record_installed(st, state_dir, doc, target, backup, resumed):
    st["installed"] = {"manifest_digest": doc["digest"], "target": target,
                       "sha256": doc["binary"]["sha256"],
                       "at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())}
    st["status"] = "installed"
    st["intent"] = None
    st["failure"] = None
    st["history"].append({"event": "installed", "target": target,
                          "sha256": doc["binary"]["sha256"], "backup": backup,
                          "resumed": resumed})
    save_state(state_dir, st)


def _fail(st, state_dir, doc, target, backup, reason):
    st["status"] = "failed"
    st["failure"] = {"reason": reason, "target": target, "backup": backup,
                     "at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())}
    st["history"].append({"event": "failed", "reason": reason, "backup": backup})
    save_state(state_dir, st)


def cmd_status(args):
    try:
        doc = load_manifest(args.state_dir)
    except ReleaseError as exc:
        print(json.dumps({"observation": "unknown_manifest", "detail": str(exc),
                          "auto_repair": False}, indent=2))
        return exc.code
    st = load_state(args.state_dir)
    installed = st.get("installed")
    target = os.path.abspath(args.target)
    actual = sha256_file(target) if os.path.exists(target) else None
    if not installed:
        print(json.dumps({"observation": "not_installed", "target": target,
                          "manifest_sha256": doc["binary"]["sha256"],
                          "actual_sha256": actual, "history": len(st.get("history", [])),
                          "auto_repair": False}, indent=2))
        return EXIT_NOT_INSTALLED
    expected = installed["sha256"]
    if actual is None:
        observation, code = "missing", EXIT_MISSING
    elif actual == expected:
        observation, code = "aligned", 0
    else:
        observation, code = "drifted", EXIT_DRIFTED
    print(json.dumps({"observation": observation, "target": target,
                      "expected_sha256": expected, "actual_sha256": actual,
                      "installed_at": installed.get("at"),
                      "installed_manifest_digest": installed.get("manifest_digest"),
                      "current_manifest_digest": doc["digest"],
                      "history": len(st.get("history", [])),
                      "auto_repair": False}, indent=2))
    return code


def build_parser():
    p = argparse.ArgumentParser(description="Beacon Go local release closure")
    sub = p.add_subparsers(dest="command", required=True)

    prep = sub.add_parser("prepare")
    prep.add_argument("--state-dir", required=True)
    prep.add_argument("--feature", required=True)
    prep.add_argument("--version", required=True)
    prep.add_argument("--requirements", default="")
    prep.add_argument("--source-commit", required=True)
    prep.add_argument("--fix", action="append")
    prep.add_argument("--build-cmd", default="")
    prep.add_argument("--build-conditions", default="")
    prep.add_argument("--binary", required=True)
    prep.add_argument("--evidence", action="append")
    prep.add_argument("--run-id", action="append")
    prep.add_argument("--maker", default="")
    prep.set_defaults(func=cmd_prepare)

    appr = sub.add_parser("approve")
    appr.add_argument("--state-dir", required=True)
    appr.add_argument("--manifest-digest", required=True)
    appr.add_argument("--actor", required=True)
    appr.set_defaults(func=cmd_approve)

    inst = sub.add_parser("install")
    inst.add_argument("--state-dir", required=True)
    inst.add_argument("--target", required=True)
    inst.add_argument("--expect-old-sha", default=None)
    inst.set_defaults(func=cmd_install)

    stat = sub.add_parser("status")
    stat.add_argument("--state-dir", required=True)
    stat.add_argument("--target", required=True)
    stat.set_defaults(func=cmd_status)
    return p


def main(argv=None):
    args = build_parser().parse_args(argv)
    try:
        result = args.func(args)
    except ReleaseError as exc:
        print(json.dumps({"error": str(exc), "code": exc.code}, indent=2), file=sys.stderr)
        return exc.code
    if isinstance(result, int):
        return result
    print(json.dumps(result, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
