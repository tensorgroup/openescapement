## Decision-model routing (pilot)

A **decision model** answers typed questions (choice, score, yes/no) about a piece of text
with probabilities, and generates no prose. It is cheap and fast enough to ask on every
task. Use one to advise three choices per task: the model tier that does the work, the
reasoning effort, and the review depth. The catalog lists the models; treat every
threshold below as a starting point to calibrate, not a validated setting.

- Keep thresholds and floors in code. Ask the decision model only for difficulty, risk,
  and whether the text is clear enough to judge. A decision model reads its instructions
  literally, and a threshold hidden in a question cannot be audited or tuned.
- Derive the tier in code from the probability that the task is hard plus a high-stakes
  signal (authentication, payments, access-control rules, private user data, production
  data migration). Do not ask the decision model to name a tier. It cannot know what a
  given tier is capable of.
- Read the upper tail or the most likely level, never the probability-weighted mean. A
  mean of "easy" can hide a one-in-five chance of "very hard".
- Floor the tier and the review depth at a low high-stakes probability. Under-routing a
  risky task costs far more than over-routing a safe one.
- Treat advice as raise-only until your own logged outcomes calibrate it. Advice may
  escalate a pinned model or deepen a review. Advice never lowers a pin, skips a mandatory
  review, or overrides a human instruction.
- Route once per task, with scoped context: phase, role, and known risks. Return no advice
  when the decision model says the text is too thin to judge. A follow-up such as "now fix
  the tests" carries none of its task's risk.
- Send only a redacted task description off the machine. Never send logs, diffs, or
  secrets. Request zero-data-retention routing. Do not log the prompt text.
- Keep routing hooks out of headless review seats. A blind seat must not see routing
  advice, and a seat's brief must not leave the machine.
- Score review findings in shadow. Hide the scores from the moderator until the verdicts
  are recorded, then join them for calibration. Exclude unverified and dropped findings
  from accuracy labels. A score shown before verification anchors the verdict it is later
  measured against.
- Log every decision with the decision model's version, the policy version, and join keys
  (session, dispatch, panel).
- Promote advice to enforcement only against criteria written down before the data is in,
  with a rollback switch. An enforcement hook that rewrites a model choice must not grant
  tool permission as a side effect.
- Test the request contract and truncation before swapping decision models. Context
  budgets and question limits differ between them, and each swap needs re-calibration.
