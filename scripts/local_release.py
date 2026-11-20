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
          Re-running never erases history: it appends a `prepared`
          event; the same digest is idempotent.
approve   Persist a human signature that matches that whole digest.
install   Lock the TARGET (shared by every state dir), check the
          expected-old preimage, persist intent + backup, atomically
          replace, verify hash + a real functional probe.
status    Compare the recorded install against the real target:
          aligned (hash AND function) / drifted / missing, with a
          stable non-zero code. Never auto-repairs.

Exit codes
----------
 0  ok / aligned
 1  operational failure
 60 drifted            61 missing
 62 unknown manifest   63 not installed
 64 stale target preimage / not approved
 65 manifest no longer matches disk (binary or evidence changed)
 66 evidence does not prove an independent QA verdict
 67 target holds unknown content during a resume (no touch)
 68 installed hash matches but the functional probe fails
"""
import argparse
import fcntl
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time

SCHEMA = "beacon-go/local-release-manifest.v1"
EXIT_DRIFTED, EXIT_MISSING = 60, 61
EXIT_UNKNOWN_MANIFEST, EXIT_NOT_INSTALLED = 62, 63
EXIT_STALE, EXIT_MANIFEST_CHANGED, EXIT_EVIDENCE_UNPROVEN = 64, 65, 66
EXIT_TARGET_DRIFT, EXIT_NOT_USABLE = 67, 68

INTEGRATED_SUBCOMMANDS = ("enable", "handoff", "dispatch", "merge", "route",
                          "qa", "approve", "finalize")

_FIELD_ALIASES = {
    "verdict": "verdict", "判定": "verdict",
    "checker": "checker", "检查者": "checker",
    "maker": "maker", "实现者": "maker",
    "candidate": "candidate", "候选": "candidate",
    "sourcecommit": "candidate", "source_commit": "candidate",
    "binary": "binary", "artifact": "binary", "产物": "binary",
    "binarysha": "binary", "binary_sha256": "binary",
}


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
    # prepared_at is a timestamp, not candidate identity: leaving it out
    # makes a same-content re-prepare idempotent.
    body = {k: v for k, v in doc.items() if k not in ("digest", "prepared_at")}
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


def new_state():
    return {"status": "prepared", "history": [], "approval": None,
            "intent": None, "installed": None, "failure": None}


def load_state(state_dir):
    _, sp = state_paths(state_dir)
    if not os.path.exists(sp):
        return new_state()
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


# --------------------------------------------------------------------------
# Evidence: structured fields, never a whole-file substring scan.
# --------------------------------------------------------------------------

def _normalize_field(name):
    key = re.sub(r"[\s_]+", "", name.strip().strip("`*|").lower())
    return _FIELD_ALIASES.get(key)


def parse_fields(text):
    """Parse `key: value` lines and `| key | value |` table rows.

    Only the declared value is used, so a verdict of FAIL cannot be
    rescued by the word PASS appearing elsewhere in the document."""
    fields = {}
    for raw in text.splitlines():
        line = raw.strip()
        if not line:
            continue
        if line.startswith("|"):
            cells = [c.strip() for c in line.strip("|").split("|")]
            if len(cells) >= 2:
                key = _normalize_field(cells[0])
                if key:
                    fields.setdefault(key, cells[1].strip("`* "))
            continue
        stripped = line.lstrip("-*+ ").strip()
        for sep in (":", "："):
            if sep in stripped:
                name, value = stripped.split(sep, 1)
                key = _normalize_field(name)
                if key:
                    fields.setdefault(key, value.strip().strip("`* "))
                break
    return fields


def verify_evidence_file(path, source_commit, binary_sha, maker):
    """Evidence must prove an INDEPENDENT QA verdict for THIS candidate.

    - `verdict` field must read PASS (a FAIL verdict elsewhere cannot be
      masked by the word PASS in prose);
    - `checker` must be present, non-empty, and different from the maker;
    - the source commit AND the binary SHA-256 must both be bound.
    """
    with open(path, encoding="utf-8", errors="replace") as f:
        text = f.read()
    body = text.lower()
    if not maker or not maker.strip():
        raise ReleaseError(EXIT_EVIDENCE_UNPROVEN,
                           "prepare needs --maker: a verdict without a maker cannot prove the checker is independent")
    fields = parse_fields(text)
    verdict = fields.get("verdict", "")
    verdict_token = verdict.split()[0].upper() if verdict.split() else ""
    if verdict_token != "PASS":
        raise ReleaseError(EXIT_EVIDENCE_UNPROVEN,
                           "evidence %s verdict is %r, not PASS (a PASS elsewhere in the body does not count)"
                           % (path, verdict or "<missing>"))
    checker = fields.get("checker", "").strip()
    if not checker:
        raise ReleaseError(EXIT_EVIDENCE_UNPROVEN, "evidence %s has no non-empty checker field" % path)
    if checker.lower() == maker.strip().lower():
        raise ReleaseError(EXIT_EVIDENCE_UNPROVEN,
                           "the maker %r cannot be its own QA checker" % maker)
    if source_commit.lower() not in body:
        raise ReleaseError(EXIT_EVIDENCE_UNPROVEN,
                           "evidence %s does not bind the source commit %s" % (path, source_commit))
    if binary_sha.lower() not in body:
        raise ReleaseError(EXIT_EVIDENCE_UNPROVEN,
                           "evidence %s does not bind the binary SHA-256 %s" % (path, binary_sha))
    return True


# --------------------------------------------------------------------------
# prepare / approve
# --------------------------------------------------------------------------

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

    mp, sp = state_paths(args.state_dir)
    st = load_state(args.state_dir)
    if st.get("intent"):
        raise ReleaseError(EXIT_STALE,
                           "an install intent is in progress for this state dir; finish or clear it before preparing a new candidate")
    previous = None
    if os.path.exists(mp):
        try:
            previous = read_json(mp)
        except (OSError, ValueError):
            previous = None
    prev_digest = previous.get("digest") if isinstance(previous, dict) else None

    if prev_digest == doc["digest"]:
        # Idempotent re-prepare: no history churn, no state reset.
        atomic_write_json(mp, doc)
        return {"prepared": True, "idempotent": True, "manifest_digest": doc["digest"],
                "binary_sha256": binary_sha}

    # A different manifest is a NEW version: keep every past event and
    # the installed record, drop only the approval (it bound the old
    # digest) and require a fresh signature.
    st["history"].append({"event": "prepared", "manifest_digest": doc["digest"],
                          "previous_digest": prev_digest,
                          "at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())})
    st["approval"] = None
    if st.get("status") != "installed":
        st["status"] = "prepared"
    st["failure"] = None
    atomic_write_json(mp, doc)
    save_state(args.state_dir, st)
    return {"prepared": True, "manifest_digest": doc["digest"],
            "previous_digest": prev_digest, "binary_sha256": binary_sha,
            "evidence": evidence, "history_events": len(st["history"])}


def recheck_disk(doc):
    """The manifest is only valid while its binary and evidence still
    hash to what was bound. Any change refuses install/resume."""
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
    if st.get("status") not in ("prepared", "approved", "installed", "failed"):
        raise ReleaseError(EXIT_STALE, "release is %s; not approvable" % st.get("status"))
    st["approval"] = {"manifest_digest": doc["digest"], "actor": args.actor,
                      "at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())}
    st["status"] = "approved"
    st["history"].append({"event": "approved", "manifest_digest": doc["digest"], "actor": args.actor})
    save_state(args.state_dir, st)
    return {"approved": True, "manifest_digest": doc["digest"], "actor": args.actor}


# --------------------------------------------------------------------------
# Target-based lock (shared by every state dir)
# --------------------------------------------------------------------------

def _lock_path(target):
    real = os.path.realpath(target)
    digest = hashlib.sha256(real.encode("utf-8")).hexdigest()[:24]
    base = os.environ.get("BEACON_RELEASE_LOCK_DIR") or os.path.join(
        os.path.expanduser("~"), ".beacon", "release-locks")
    os.makedirs(base, exist_ok=True)
    return os.path.join(base, "target-" + digest + ".lock")


def _lock(target):
    f = open(_lock_path(target), "w")
    fcntl.flock(f.fileno(), fcntl.LOCK_EX)
    return f


# --------------------------------------------------------------------------
# Functional probe: version + integrated entry + isolated temp chain
# --------------------------------------------------------------------------

def functional_probe(binary):
    """A binary is usable only when it answers --version, exposes the
    integrated delivery subcommands, and can open an ISOLATED store and
    reach the integrated route entry. The production store is never
    touched; the probe store lives under a temp dir and is discarded."""
    try:
        proc = subprocess.run([binary, "--version"], capture_output=True, text=True, timeout=20)
    except (OSError, subprocess.TimeoutExpired) as exc:
        return False, "version probe failed: " + str(exc)
    if proc.returncode != 0:
        return False, "version probe exit %d" % proc.returncode
    try:
        help_proc = subprocess.run([binary, "goal", "program", "integrated", "--help"],
                                   capture_output=True, text=True, timeout=20)
    except (OSError, subprocess.TimeoutExpired) as exc:
        return False, "integrated help probe failed: " + str(exc)
    if help_proc.returncode != 0:
        return False, "integrated help exit %d" % help_proc.returncode
    missing = [sub for sub in INTEGRATED_SUBCOMMANDS
               if sub not in (help_proc.stdout + help_proc.stderr)]
    if missing:
        return False, "integrated help is missing: " + ", ".join(missing)
    with tempfile.TemporaryDirectory(prefix="beacon-probe-") as tmp:
        env = dict(os.environ, BEACON_STORE_PATH=os.path.join(tmp, "probe.db"))
        try:
            route_proc = subprocess.run(
                [binary, "goal", "program", "integrated", "route",
                 "--run-id", "release-probe-missing-run", "--json"],
                capture_output=True, text=True, timeout=30, env=env)
        except (OSError, subprocess.TimeoutExpired) as exc:
            return False, "integrated route probe failed: " + str(exc)
    out = (route_proc.stdout + route_proc.stderr).strip()
    if route_proc.returncode == 0:
        return False, "integrated route probe unexpectedly found a run in a fresh store"
    if "unknown command" in out.lower():
        return False, "integrated route entry is not wired"
    return True, "version + integrated entry + isolated route probe ok"


# --------------------------------------------------------------------------
# install
# --------------------------------------------------------------------------

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


def cmd_install(args):
    target = os.path.abspath(args.target)
    os.makedirs(os.path.dirname(target), exist_ok=True)
    lock = _lock(target)
    try:
        doc = load_manifest(args.state_dir)
        st = load_state(args.state_dir)
        candidate = doc["binary"]["sha256"]
        current = sha256_file(target) if os.path.exists(target) else None

        # 1) Approval and manifest integrity are checked on EVERY path,
        #    including a crash resume: swapped evidence/binary stops it.
        approval = st.get("approval")
        if not approval or approval.get("manifest_digest") != doc["digest"]:
            raise ReleaseError(EXIT_STALE, "release is not approved for this manifest digest (status=%s)"
                               % st.get("status"))
        recheck_disk(doc)

        intent = st.get("intent")
        resuming = bool(intent) and intent.get("manifest_digest") == doc["digest"] and \
            os.path.realpath(intent.get("target", "")) == os.path.realpath(target)

        if resuming:
            old = intent.get("old_sha256")
            # 2) Only two target states are ours: the exact preimage we
            #    recorded, or the candidate we already wrote. Anything
            #    else is unknown content: report drift, touch nothing.
            if current not in (old, candidate):
                raise ReleaseError(EXIT_TARGET_DRIFT,
                                   "resume found unknown target content (%s); expected %s or %s — refusing to overwrite"
                                   % (current, old, candidate))
            if args.expect_old_sha is not None and args.expect_old_sha not in (old, candidate):
                raise ReleaseError(EXIT_STALE,
                                   "resume preimage %s matches neither the recorded old (%s) nor the candidate (%s)"
                                   % (args.expect_old_sha, old, candidate))
            if current == candidate:
                ok, detail = functional_probe(target)
                if not ok:
                    _fail(st, args.state_dir, doc, target, intent.get("backup"),
                          "functional probe failed: " + detail)
                    raise ReleaseError(1, "installed binary failed its functional probe: " + detail)
                _record_installed(st, args.state_dir, doc, target, intent.get("backup"), resumed=True)
                return {"installed": True, "resumed": True, "target": target,
                        "sha256": current, "manifest_digest": doc["digest"],
                        "backup": intent.get("backup")}
            # Replace had not happened yet: continue with the recorded
            # preimage (no new approval, backup already recorded).
            backup = intent.get("backup") or _backup(target, args.state_dir,
                                                     time.strftime("%Y%m%dT%H%M%S") + "-" + str(os.getpid()))
            intent["backup"] = backup
            st["intent"] = intent
            st["status"] = "installing"
            save_state(args.state_dir, st)
        else:
            # 3) Fresh install: the expected preimage is mandatory for an
            #    existing target — never overwrite unknown content.
            if current is not None:
                if args.expect_old_sha is None:
                    raise ReleaseError(EXIT_STALE,
                                       "target exists (sha %s); --expect-old-sha is required to overwrite it" % current)
                if args.expect_old_sha != current:
                    raise ReleaseError(EXIT_STALE,
                                       "expected old target sha256 %s but found %s; refusing stale overwrite"
                                       % (args.expect_old_sha, current))
            elif args.expect_old_sha is not None and args.expect_old_sha != "":
                raise ReleaseError(EXIT_STALE,
                                   "target does not exist but --expect-old-sha %s was given" % args.expect_old_sha)
            if st.get("status") == "installed" and current == candidate and \
                    (args.expect_old_sha in (None, candidate)):
                return {"installed": True, "already": True, "target": target,
                        "sha256": current, "manifest_digest": doc["digest"]}
            stamp = time.strftime("%Y%m%dT%H%M%S") + "-" + str(os.getpid())
            backup = _backup(target, args.state_dir, stamp)
            intent = {"manifest_digest": doc["digest"], "target": target,
                      "old_sha256": current, "candidate_sha256": candidate,
                      "backup": backup,
                      "at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())}
            st["intent"] = intent
            st["status"] = "installing"
            st["history"].append({"event": "install_intent", "target": target,
                                  "old_sha256": current, "backup": backup})
            save_state(args.state_dir, st)

        _atomic_replace_source(doc["binary"]["path"], target)

        if os.environ.get("BEACON_RELEASE_CRASH_AFTER_REPLACE") == "1":
            # Test hook: simulate a crash between replace and record.
            os._exit(70)

        if sha256_file(target) != candidate:
            _fail(st, args.state_dir, doc, target, intent.get("backup") if intent else None,
                  "target hash differs after install")
            raise ReleaseError(1, "installed target hash does not match the manifest")
        ok, detail = functional_probe(target)
        if not ok:
            _fail(st, args.state_dir, doc, target, intent.get("backup") if intent else None,
                  "functional probe failed: " + detail)
            raise ReleaseError(1, "installed binary failed its functional probe: " + detail)
        _record_installed(st, args.state_dir, doc, target,
                          intent.get("backup") if intent else None, resumed=resuming)
        return {"installed": True, "target": target, "sha256": candidate,
                "manifest_digest": doc["digest"],
                "backup": intent.get("backup") if intent else None}
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


# --------------------------------------------------------------------------
# status
# --------------------------------------------------------------------------

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
    probe_ok, probe_detail = (None, "not run")
    if actual is None:
        observation, code = "missing", EXIT_MISSING
    elif actual != expected:
        observation, code = "drifted", EXIT_DRIFTED
    else:
        # A hash match alone is NOT delivery: the installed binary must
        # answer the real integrated entry (isolated temp store).
        probe_ok, probe_detail = functional_probe(target)
        if probe_ok:
            observation, code = "aligned", 0
        else:
            observation, code = "not_usable", EXIT_NOT_USABLE
    print(json.dumps({"observation": observation, "target": target,
                      "expected_sha256": expected, "actual_sha256": actual,
                      "functional_probe": probe_ok, "functional_detail": probe_detail,
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
    prep.add_argument("--maker", required=True)
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
