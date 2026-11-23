# Stateless draft decomposition example

Use a disposable project and explicit store for experiments; do not point examples at a production ledger.

```sh
export BEACON_STORE_PATH=/tmp/beacon-decompose-example.db
mkdir -p /tmp/beacon-decompose-example
beacon-go projects register --path /tmp/beacon-decompose-example --version v0.0.1
beacon-go truth create --project /tmp/beacon-decompose-example --slug format --sdd
beacon-go truth decompose --project /tmp/beacon-decompose-example --feature format --utterance '报价保留两位小数' --json
```

The JSON reports `method: deterministic_non_llm` and `semantic_ready: false`. The `Decomposition Suggestions` section contains candidate clauses and any keyword-matched journey seeds. The authored Acceptance Criteria, Grounding, Open Questions and task graph are not replaced. Only truth.md and tasks.md remain authored members; decomposition must not create tests.md.

At this point the following command must exit nonzero:

```sh
beacon-go truth freeze --project /tmp/beacon-decompose-example --feature format --by example-owner
```

This refusal is the expected result, not a failed example. The user and external agent must ground and write actual acceptance behavior, meaningful tests and an executable DAG, resolve applicable questions and obtain a real independent semantic review. Rule matching cannot do that work or sign the package.

For a workflow utterance, candidate journey seeds may suggest actors and steps. Those seeds are explicitly suggestions; use the actual domain's transitions and test coverage, not a generic Draft/Active/Done lifecycle. See the two-file contract in docs/sdd-truth.md for the review and confirmation chain.
