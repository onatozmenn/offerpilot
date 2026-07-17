# Online-Learning Design

## Objective

For each validated session context, choose one active offer and maximize expected bounded reward while retaining controlled exploration and auditable action probabilities.

## Why The MVP Uses Segmented Epsilon-Greedy

The policy is simple enough to explain and test, supports incremental updates, and yields exact propensities needed for off-policy evaluation. More sophisticated algorithms are deferred until the logging and evaluation foundation is trustworthy.

See [ADR-002](decisions/ADR-002-policy-and-propensity.md).

## Policy Interface

A policy receives canonical context plus a deterministically ordered set of active offers and returns:

- Selected offer ID.
- Probability for every eligible offer.
- Policy kind and current version.

An update receives the original decision metadata, one server-derived reward, and the consecutive application version reserved by storage. A delayed decision selected at an older policy version remains valid; the policy rejects only an application version that is not exactly current version plus one. Policies support snapshot and restore. Random sources are injected and safe for concurrent use.

## Segment Construction

The canonical key is:

```text
device_class|daypart|category_affinity|visitor_type
```

Each component is a validated enum. This intentionally coarse representation avoids identity and sparse high-cardinality data.

## Random Baseline

For $K$ eligible offers, each action has probability:

$$
p(a) = \frac{1}{K}
$$

The baseline does not learn, but each accepted feedback advances its application version and produces a new minimal snapshot. This keeps recovery and outcome attribution identical across policy kinds while leaving the uniform distribution unchanged.

## Segmented Epsilon-Greedy

For each segment-offer pair, maintain a weighted count $n$ and cumulative reward $s$. The empirical mean is:

$$
\hat{\mu} = \frac{s}{n}
$$

Cold-start state uses prior count $n_0 = 2$ and prior reward sum $s_0 = 1$, corresponding to a neutral mean of $0.5$.

Let $K$ be eligible offers and $B$ the number tied for best empirical mean. For exploration rate $\epsilon$:

$$
p(a) =
\begin{cases}
\frac{\epsilon}{K} + \frac{1-\epsilon}{B}, & a \text{ is tied for best} \\
\frac{\epsilon}{K}, & \text{otherwise}
\end{cases}
$$

The action is sampled from this full distribution. The exact selected probability is persisted as propensity.

On accepted feedback with reward $r$, at the next consecutive application version:

$$
n \leftarrow n + 1, \qquad s \leftarrow s + r
$$

The version advances exactly once per accepted outcome. The decision's selection version can be older because feedback is allowed to arrive after later decisions.

## Numerical Rules

- Epsilon and rewards are finite and bounded to `[0, 1]`.
- Offers are sorted by UUID before scoring and sampling.
- Probability sum tolerance is `1e-9`.
- Sampling uses cumulative probabilities and a uniform draw in `[0, 1)`.
- Snapshots use integer schema version `1` and include policy version, epsilon, priors, and all segment-offer statistics.
- Invalid or corrupt snapshots are rejected rather than partially restored.

## Synthetic Outcome Model

The simulator contains hidden, seeded affinity weights for context-offer combinations. It computes outcome probabilities, draws a terminal outcome, and accumulates simulation-only observed, uniform-random expected, and oracle expected reward sums on the simulation run. Production-facing policy code cannot access hidden reward probabilities.

The simulator must be deterministic for the same seed, configuration, and request order. It must not imply that synthetic uplift transfers to real users.

## Off-Policy Evaluation

Every logged decision includes the behavior-policy probability $\pi_b(a|x)$. For a candidate policy $\pi_e$, IPS estimates value as:

$$
\hat{V}_{IPS} = \frac{1}{N}\sum_{i=1}^{N} r_i\frac{\pi_e(a_i|x_i)}{\pi_b(a_i|x_i)}
$$

Self-normalized IPS divides the weighted reward sum by the weight sum. Evaluation rejects zero or invalid behavior propensities and reports effective sample size. Estimates are reportable when effective sample size is at least `10`; otherwise they are null with `no_samples`, `zero_candidate_weight`, or `low_effective_sample_size` as the stable reason.

Direct-method and doubly robust estimators are roadmap items because they require a separately validated reward model.

## Model Lifecycle

- Restore latest valid snapshot before readiness succeeds.
- Persist a checkpoint after each accepted feedback in the MVP; optimize batching only after measurement.
- Associate each decision with the version used to select it.
- Associate each outcome with the consecutive application version reserved by feedback acceptance order.
- Never mutate historical distributions after a policy update.

## Deferred Algorithms

LinUCB, Thompson Sampling, slate optimization, delayed-reward attribution, and non-stationarity decay require new ADRs and file specs. They are not hidden MVP requirements.
