#!/usr/bin/env python3
"""Real CLI fixtures in disposable git repositories/stores; never seed a DB.

BEACON_BIN selects an already built binary. BEACON_EVIDENCE_DIR keeps the
complete command trail; --cross-batch runs the original dispatch counterexample.
"""
import concurrent.futures
import datetime
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

BIN = Path(os.environ['BEACON_BIN']).resolve()
OUT = Path(os.environ.get('BEACON_EVIDENCE_DIR', tempfile.mkdtemp(prefix='program-cli-evidence-')))
OUT.mkdir(parents=True, exist_ok=True)
WORK = Path(tempfile.mkdtemp(prefix='program-cli-'))
REPO = WORK / 'repo'
REPO.mkdir()
ENV = dict(os.environ, BEACON_STORE_PATH=str(WORK / 'isolated.db'))
records = []

def call(argv, expect=0, cwd=REPO):
    p = subprocess.run(list(map(str, argv)), cwd=cwd, env=ENV, capture_output=True, text=True, timeout=30)
    row = dict(argv=list(map(str, argv)), cwd=str(cwd), exit=p.returncode, stdout=p.stdout,
               stderr=p.stderr, at=datetime.datetime.now(datetime.timezone.utc).isoformat())
    records.append(row)
    (OUT / 'commands.json').write_text(json.dumps(records, indent=2, ensure_ascii=False))
    assert p.returncode == expect, row
    return json.loads(p.stdout) if p.stdout.lstrip().startswith('{') else p.stdout.strip()

def cli(*args, **kw):
    return call([BIN, *args, '--json'], **kw)

def git(*args, **kw):
    return call(['git', *args], **kw)

def integrated(action, expect=0, **kw):
    args = ['goal', 'program', 'integrated', action, '--run-id', host]
    for key, value in kw.items():
        args += ['--' + key.replace('_', '-'), str(value)]
    return cli(*args, expect=expect)

git('init', '-q', '-b', 'main')
git('config', 'user.name', 'isolated-fixture')
git('config', 'user.email', 'fixture@local')
(REPO / 'README.md').write_text('Disposable program CLI fixture\n')
(REPO / '.gitignore').write_text('.beacon/\nqa/\n')
git('add', '.')
git('commit', '-qm', 'initial fixture')
cli('projects', 'register', '--path', REPO, '--version', 'v1.0.0')

# Reuse the source-controlled E2E package authoring block, not a past temp dir.
source = (Path(__file__).parent / 'run_e2e.sh').read_text()
author = source.split('PKG_DIR="$PKG_DIR" python3 - <<\'PYEOF\'\n', 1)[1].split('\nPYEOF', 1)[0]
slugs = ['host', 'b', 'c', 'f', 'e'] if '--cross-batch' in sys.argv else ['host', 'a', 'b', 'c', 'd']
for slug in slugs:
    cli('truth', 'create', '--project', REPO, '--slug', slug)
    package = REPO / 'docs/beacon/v1.0.0/features' / slug
    old = os.environ.get('PKG_DIR')
    os.environ['PKG_DIR'] = str(package)
    exec(author.replace('e2e-login-2fa', slug), {})
    if old is None:
        del os.environ['PKG_DIR']
    else:
        os.environ['PKG_DIR'] = old
    cli('truth', 'intent', '--project', REPO, '--feature', slug, '--utterance', 'fixture login TOTP',
        '--goal', 'fixture login rejects invalid codes', '--non-goals', 'no SMS', '--constraints', 'offline', '--scope-mode', 'focused')
    cli('truth', 'answer', '--project', REPO, '--feature', slug, '--id', 'Q1', '--answer', 'fixture login rejects invalid codes')
    cli('truth', 'confirm', '--project', REPO, '--feature', slug, '--by', 'fixture-author')
    cli('truth', 'freeze', '--project', REPO, '--feature', slug, '--by', 'fixture-author')
git('add', 'docs')
git('commit', '-qm', 'freeze fixture contracts')
for slug in slugs:
    cli('truth', 'baseline', '--project', REPO, '--feature', slug, '--by', 'fixture-author')
    topology = WORK / ('topology-' + slug + '.json')
    topology.write_text(json.dumps(dict(schema_version='git-scope-graph.v1', project_topology='single_repo',
        scopes=[dict(scope_id='primary', git_root=str(REPO), role='primary', dirty_policy='fail_closed')])))
    cli('goal', 'topology', 'project', '--project', REPO, '--feature', slug, '--graph', topology, '--by', 'fixture-author')
    if slug != 'host':
        cli('goal', 'run', 'fixture lake', '--project', REPO, '--feature', slug,
            '--executor', 'manual', '--executor-id', 'fixture-maker')
host = cli('goal', 'run', 'fixture program', '--project', REPO, '--feature', 'host',
           '--executor', 'manual', '--executor-id', 'fixture-maker')['run_id']
if '--cross-batch' in sys.argv:
    lakes = [dict(slug=s, touches=t, depends_on=[]) for s, t in
             [('b', ['shared']), ('c', ['other']), ('f', ['shared']), ('e', [])]]
else:
    lakes = [dict(slug=s, touches=[s], depends_on=deps) for s, deps in
             [('a', []), ('b', ['a']), ('c', ['a']), ('d', ['b', 'c'])]]
graph = WORK / 'graph.json'
for lake in lakes:
    lake.update(parity_subset=[], status='selected')
graph.write_text(json.dumps(dict(program_slug='fixture', version='v1.0.0', scope_mode='lake', lakes=lakes)))
cli('goal', 'program', 'import', '--run-id', host, '--file', graph, '--freeze')
integrated('enable')
if '--cross-batch' in sys.argv:
    integrated('dispatch')
    second = integrated('dispatch')
    third = integrated('dispatch')
    assert not second.get('dispatched') and not third.get('dispatched'), (second, third)
    # Concurrent independent CLI processes must retain the same host snapshot.
    with concurrent.futures.ThreadPoolExecutor() as pool:
        results = list(pool.map(lambda _: integrated('dispatch'), range(2)))
    integrated('dispatch')
    view = integrated('route')
    active = {d['lake'] for d in view['dispatches'] if d['status'] == 'active'}
    assert active == {'b', 'c'}, (results, view)
    print(json.dumps(dict(result='cross-batch success', host=host, workspace=str(WORK),
                          binary_sha256=hashlib.sha256(BIN.read_bytes()).hexdigest())))
    sys.exit(0)

def gate(slug, run, kind, expect=0):
    return cli('gate', 'evaluate', kind, '--project', REPO, '--feature', slug,
               '--run', run, '--evaluator', 'fixture-checker', expect=expect)

def finish_stages(slug, run):
    topology = WORK / ('topology-' + slug + '.json')
    topology.write_text(json.dumps(dict(schema_version='git-scope-graph.v1', project_topology='single_repo',
        scopes=[dict(scope_id='primary', git_root=str(REPO), role='primary', dirty_policy='fail_closed')])))
    cli('goal', 'topology', 'project', '--project', REPO, '--feature', slug, '--graph', topology, '--by', 'fixture-author')
    gate(slug, run, 'truth_freeze')
    for _ in range(8):
        state = cli('goal', 'status', '--run-id', run)
        if state['loop_stage'] == 'release_gate':
            break
        stage = state['current_stage']
        if stage == 'qa':
            # Actual fixture checks precede reports and registration.
            assert (REPO / (slug + '.txt')).read_text() == slug + '\n'
            report = WORK / (slug + '-qa.md')
            report.write_text('Fixture assertions passed for ' + run + ': committed work product matches.\n')
            cli('goal', 'artifacts', 'register', '--run-id', run, '--kind', 'qa_report', '--path', report, '--by', 'fixture-checker')
            surface = WORK / (slug + '-e2e.md')
            surface.write_text(report.read_text() + 'CLI surface exercised through tick and gates.\n')
            cli('goal', 'artifacts', 'register', '--run-id', run, '--kind', 'e2e_report', '--path', surface, '--by', 'fixture-checker')
            autotest = REPO / '.beacon/autotest' / run
            autotest.mkdir(parents=True, exist_ok=True)
            (autotest / 'report.json').write_text(json.dumps(dict(run_id=run, assertion='work product matches', failed=0)))
        tick = cli('goal', 'tick', '--run-id', run, '--by', 'fixture-checker' if stage == 'qa' else 'fixture-maker')
        assert not tick.get('blocked'), tick
    else:
        raise AssertionError('stages did not reach release gate')
    for kind in ['design', 'implement', 'qa']:
        gate(slug, run, kind)
    cli('goal', 'attach-verifier', '--run-id', run, '--source', 'fixture-checker', '--ref', report)
    return report

runs, reports = {}, {}
for batch in [('a',), ('b', 'c'), ('d',)]:
    dispatch = integrated('dispatch')
    assert {d['lake'] for d in dispatch['dispatched']} == set(batch), dispatch
    for d in dispatch['dispatched']:
        slug, run, path = d['lake'], d['run_id'], Path(d['worktree_path'])
        runs[slug] = run
        for dep in next(l['depends_on'] for l in lakes if l['slug'] == slug):
            assert (path / (dep + '.txt')).read_text() == dep + '\n'
        (path / (slug + '.txt')).write_text(slug + '\n')
        git('add', slug + '.txt', cwd=path)
        git('commit', '-qm', 'fixture work ' + slug, cwd=path)
        src = git('rev-parse', 'HEAD', cwd=path)
        integrated('merge', lake=slug)
        reports[slug] = finish_stages(slug, run)
        integrated('handoff', lake=slug, source_sha=src, integration_sha=git('rev-parse', 'HEAD'))
    if batch == ('a',):
        original = reports['a'].read_bytes()
        reports['a'].write_text('tampered evidence\n')
        view = integrated('route')
        assert view['blocked']['b'] == 'program_handoff_invalid', view
        for extra in [[], ['--until-done', '--max-duration-sec', '1']]:
            view = cli('goal', 'program', 'drive', '--run-id', host, '--executor', 'manual', *extra)
            assert view.get('advanced_lake') not in ('b', 'c', 'd'), view
            assert all(row.get('reason_code') == 'program_handoff_invalid' for row in view['route']['lakes'] if row['lake'] in ('b', 'c')), view
        reports['a'].write_bytes(original)

# The host is a real run with its own normal stage/gate evidence.
(REPO / 'host.txt').write_text('host\n')
git('add', 'host.txt')
git('commit', '-qm', 'fixture host integration checks')
reports['host'] = finish_stages('host', host)
integrated('qa', evidence=WORK / 'missing.md', by='fixture-checker', expect=1)
total = WORK / 'total.md'
assert all((REPO / (s + '.txt')).read_text() == s + '\n' for s in runs)
total.write_text('Diamond assertions passed: A, B, C, D work products and dependency baselines checked.\n')
candidate = integrated('qa', evidence=total, by='fixture-checker')['candidate']
total.write_text('changed total evidence\n')
integrated('approve', candidate_sha=candidate['sha'], evidence_digest=candidate['evidence_digest'], confirmed_by='fixture-human', expect=1)
total.write_text('Diamond assertions passed: A, B, C, D work products and dependency baselines checked.\n')
approved = integrated('approve', candidate_sha=candidate['sha'], evidence_digest=candidate['evidence_digest'], confirmed_by='fixture-human')
total.write_text('changed after approval\n')
integrated('finalize', expect=1)
total.write_text('Diamond assertions passed: A, B, C, D work products and dependency baselines checked.\n')
# Force a genuine partial finalize stop at B's formal release gate, after A
# closes; restore the gate through official CLI, then resume the same signature.
for _ in range(3):
    cli('goal', 'block', '--run-id', runs['b'], '--reason-code', 'fixture_pause', expect=1)
gate('b', runs['b'], 'implement', expect=1)
integrated('finalize', expect=1)
assert cli('goal', 'status', '--run-id', runs['a'])['loop_stage'] == 'done'
cli('goal', 'recover', '--run-id', runs['b'], '--by', 'fixture-maker', '--note', 'fixture pause resolved')
gate('b', runs['b'], 'implement')
done = integrated('finalize')
assert done['stage'] == 'delivered' and done['approval'] == approved['approval'], done
integrated('finalize')
for run in [*runs.values(), host]:
    assert cli('goal', 'status', '--run-id', run)['loop_stage'] == 'done'
    trace = call([BIN, 'goal', 'trace', '--run-id', run])
    assert sum(ev['event_type'] == 'release_confirmed' for ev in trace['events']) == 1, trace
print(json.dumps(dict(result='diamond success with rejection and partial resume', host=host, runs=runs,
                      workspace=str(WORK), binary_sha256=hashlib.sha256(BIN.read_bytes()).hexdigest())))
