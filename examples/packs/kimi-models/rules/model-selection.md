## Choosing a Kimi model

Kimi K3.1 Pro is the model for judgment: plans, plan checks, and reviews. Its High-Speed variant is the same model, faster and dearer.

| Model | ID (API) | ID (Kimi Code) | $/1M tokens (in/out), 2026-10-24 | Use for |
|---|---|---|---|---|
| Kimi K3.1 Pro | `kimi-k3.1-pro` | `kimi-code/k3.1-pro` | $2 / $10 | Planning, plan-check, review, day-to-day coding. |
| Kimi K3.1 Pro High-Speed | `kimi-k3.1-pro-highspeed` | `kimi-code/k3.1-pro-highspeed` | $4 / $20 | Fast reads, rapid iteration. |
| Kimi K3 | `kimi-k3` | `kimi-code/k3` | $3 / $15 | Existing pins only; migrate to K3.1 Pro. |
| Kimi K2.7 Code | `kimi-k2.7-code` | | $0.95 / $4.00 | Existing pins only; the K2 series retires by the end of December 2026. |

Rules of thumb:

- Run an unattended Kimi Code session (`kimi -p`) under an OS sandbox that denies writes outside its working directory. Non-interactive runs use the `auto` permission mode, deny rules did not stop writes in a 2026-10-24 test of Kimi Code 2.3.1, and hooks fail open by Kimi Code's own documentation.
- Treat a Kimi Code hook as an alert, not a barrier. A hook that errors or times out allows the action.
- Keep Kimi Code plan billing and Kimi API Platform billing apart. The two use different keys, and a key from one does not work on the other.
- Move K2-series pins to K3.1 Pro before the end of December 2026. New accounts lose K2 access on 2026-10-31.
- Model lineup and pricing change. This guidance is versioned and updated centrally; changes arrive through the pack, not by editing this block.
