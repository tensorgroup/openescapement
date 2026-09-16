## Model seat policy

Multiple frontier models participate in reviews and planning as **seats** with assigned
duties. A seat is a role; the model behind it is one line of config, expected to change
(the reference roster's first metered model was retired by its provider within weeks).
Seats are granted and revoked from *logged evidence*, dispute win shares and
confirmed-finding rates, never from a single anecdote or a model's confidence.

| Seat | Duties | Restrictions |
|---|---|---|
| Moderator (the primary agent) | frame the brief, run the seats, moderate disputes, verify findings against code, rule, log | may overrule any seat; may not skip logging; may not favor its own vendor's seat |
| Planning lead | lead voice on plan, design, and decision targets; equal peer on code review | read-only access, always |
| Peer seats | independent review on every target kind | read-only access, always |
| On-demand seats | convened for high-stakes targets or when a second blind read from a family is worth the wall-clock | scored like every other seat when they run |

Conduct rules the moderator enforces:

- **Substance bar.** A seat's challenge earns a rebuttal round only if it disputes facts,
  behavior, or severity with concrete evidence ("what breaks, where"). Wording, style,
  and degree-of-certainty objections are dismissed without being relayed, and are still
  logged as disputes, scored against the challenger.
- **Rebuttal caps.** Two rounds. Convergence is not required; the moderator rules on
  unresolved disputes.
- **Read-only seats.** External model seats never hold write access to a checkout.
  Verify by attempted write, on fresh and resumed sessions, before trusting a seat and
  again after any harness upgrade.
- **Wall-clock caps.** Every seat runs under a finite cap enforced by the launcher, not
  by the model. A seat that does not return inside it is recorded as absent, left out
  of that panel's per-seat counts, and the panel proceeds on the rest; a timeout never
  drops a seat from the roster.
- **Evidence-cited policy changes.** Any change to seat assignments must cite the
  scoreboard (win share, confirmed-finding rate, unique catches) over a window, never
  one bad review, and state what would earn the role back. This block is versioned
  policy; changes arrive through the pack, not by editing rendered files.
