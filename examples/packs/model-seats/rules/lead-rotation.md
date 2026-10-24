## Lead rotation

A **profile** is a project's ordered leads, the seats for each checkpoint, the required seats, and the minimum seat count. The first eligible lead in that order runs the project; the others stand by for when its plan runs out. Rotation moves the lead between plans the user already pays for, and the evidence for which lead is best comes from the record, not from the leads.

- Name the leads per profile, in order of preference. A profile with one lead does not rotate.
- Run a lead only under containment verified both ways by attack. A write inside the project's worktree must succeed, and a write outside it must fail with an OS error. A lead writes, so an unverified lead can damage anything the user can.
- Re-run that attack whenever the lead's runtime version or containment configuration changes. A runtime that has not passed on its current version is not eligible.
- Switch away from a lead when its plan cap is observed. The capped session cannot continue, so switching loses nothing.
- Return to the preferred lead at the next task boundary after its recorded reset time. Never switch mid-turn, because a half-finished turn cannot be handed off cleanly.
- Hand off from facts the next lead can check: the branch, `git status`, the commits since the session started, and the last checkpoint the outgoing lead wrote. Keep handoffs outside the repo.
- Never run two writable leads in one worktree. Hold a lock on the worktree for as long as any lead runs there.
- Log every switch with from, to, and reason, each lead named by resolved model id. A switch the user cannot reconstruct afterwards is an incident.
- Score leads from git, from confirmed review findings on their commits, and from the user's rating. Never score a lead from its own report of its work.
- Keep a lead's scores per dimension (throughput, cost, defects caught in review, escapes, human load). Do not combine them into one lead score, because a composite hides which trade-off moved.
