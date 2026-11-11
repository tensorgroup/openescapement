## Model seat policy

Multiple frontier models participate in reviews and planning as **seats** with assigned
duties. A seat is a role; the model behind it is one line of config, expected to change
(the reference roster's first metered model was retired by its provider within weeks).
Seats are granted and revoked from logged evidence, dispute win shares and
confirmed-finding rates, never from a single anecdote or a model's confidence.

The **lead** is the agent the user works in for a project; the lead is the moderator of every panel it convenes.

| Seat | Duties | Restrictions |
|---|---|---|
| Moderator (the lead) | frame the brief, run the seats, moderate disputes, verify findings against code, rule, log | may overrule any seat; may not skip logging; may not favor its own vendor's seat; never scores itself |
| Planning seat | first voice on plan, design, and decision targets; equal peer on code review | read-only access, always |
| Peer seats | independent review on every target kind | read-only access, always |
| On-demand seats | convened for high-stakes targets or when a second blind read from a family is worth the wall-clock | scored like every other seat when they run |
| Reserved seats | join deep panels and stuck debugging only, on a recorded reason | the most expensive model; never in a default panel; scored like every other seat when they run |

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
- **OS-level enforcement.** Enforce read-only access with an OS sandbox. A seat's own
  permission settings enforce nothing, because vendor permission rules have been seen to
  ignore deny rules in non-interactive mode. Prove the sandbox by attack: an attempted
  write must fail with an OS error on a fresh and on a resumed session.
- **Recorded moderator.** Record which lead moderated each panel, by resolved model id.
  A lenient moderator inflates every seat's confirmed rate, and an alias hides which model
  ruled.
- **Same-vendor seats.** Seat the lead's own vendor only as a fresh blind session with
  none of the lead's conversation. Exclude any seat that shares the lead's session, since
  the moderator would be reviewing its own reasoning.
- **Wall-clock caps.** Every seat runs under a finite cap enforced by the launcher, not
  by the model. A seat that does not return inside it is recorded as absent, left out
  of that panel's per-seat counts, and the panel proceeds on the rest; a timeout never
  drops a seat from the roster.
- **Usage caps.** When a review seat's model hits a plan usage cap, run that seat on its
  configured fallback model and score the run under the fallback's own id. Record the
  capped seat as absent (usage cap), because crediting one model with another's findings
  corrupts the scoreboard the roster is decided from. A capped lead follows the lead
  rotation rule (`rules/lead-rotation.md`) instead.
- **Cost order.** Fill the default panel from the cheapest seats that do the job. The default order is the lead's default model, then the planning seat from another vendor, then the peer seats. Escalate to a reserved seat only for a deep panel or stuck debugging, because a cross-vendor check usually adds more than a more expensive model from the same vendor.
- **Evidence-cited policy changes.** Any change to seat assignments must cite the
  scoreboard (win share, confirmed-finding rate, unique catches) over a window, never
  one bad review, and state what would earn the role back. This block is versioned
  policy; changes arrive through the pack, not by editing rendered files.
