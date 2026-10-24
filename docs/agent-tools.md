# Agent tools an org governs

*Written 2026-10-24. Tool versions named here were current on that date: Claude Code 2.1.310,
Codex CLI 0.154.0, agy 1.4.7, pi 0.85.1, Kimi Code CLI 2.3.1, nono 0.80.0, cc-switch 4.0.15.
Prices move monthly; the vendor notes in the portal's model guidance carry the dated
numbers and their sources, and this page links to them instead of repeating them.*

This page covers what an org decides about the agent tools its people run: what each tool
is, what it costs, and the one setting that matters for governance. It does not cover
per-user setup. [Balancewheel](https://github.com/tensorgroup/balancewheel) owns per-user
agent configuration (seat homes, wrappers, sandboxes for review seats), and its README
is where a developer goes to install and wire these tools. `esc` writes agent-readable
files only in a repo; Balancewheel writes only under the user's home directory.

## The vendor CLIs

Each vendor ships its own repo-aware agent. Installing one is free. The cost is the plan
or the API key it signs in with.

| CLI | Vendor | Billed to | The setting that matters for governance |
|---|---|---|---|
| `claude` (Claude Code) | Anthropic | Claude Pro or Max plan, or an API key | Which models the `opus` and `sonnet` aliases resolve to; pin them, or a fallback can land on a model your policy rules out. |
| `codex` (Codex CLI) | OpenAI | ChatGPT Plus or Pro plan, or an API key | The sandbox mode. Review use needs read-only; a lead needs workspace-write, contained to the worktree. |
| `agy` (Antigravity CLI) | Google | Google AI Pro or Ultra plan | agy writes a cache file into its working directory, so a read-only use must run it from a directory outside the reviewed repo. It also serves Claude Opus 5.5 and Sonnet 5.5 on the Google plan. |
| `kimi` (Kimi Code CLI) | Moonshot AI | Kimi Code plan, or a Kimi API Platform key | Its permission rules are not enforcement in non-interactive (`-p`) runs; see [the sandboxes](#os-level-containment-nono-and-sandbox-exec). |
| `pi` | open source (MIT) | Any provider key, typically OpenRouter | None of its own policy: pi runs whatever model its provider config names, so the provider key's spend cap is the control. In `-p` mode it waits for stdin unless stdin is closed. |

## Plans versus metered keys

A plan bills a flat monthly price and meters usage against windows the vendor sets. A
metered key bills every token. For a person who codes with an agent most days, the
vendor's own CLI on a plan is usually the cheaper route; metered keys suit fast, small,
occasional calls and unattended jobs that must not drain a person's plan. Each vendor note
has an Access routes section with the dated prices and the vendor's stated limits. A note
whose vendor sells a subscription plan with a published price that covers its CLI also
works out where that plan breaks even:

- [Anthropic](../internal/guidance/models/anthropic.md#access-routes)
- [OpenAI](../internal/guidance/models/openai.md#access-routes)
- [Google](../internal/guidance/models/google.md#access-routes)
- [Kimi](../internal/guidance/models/kimi.md#access-routes)
- [DeepSeek](../internal/guidance/models/deepseek.md#access-routes)
- [Grok](../internal/guidance/models/grok.md#access-routes)

## OpenRouter

One key reaches every vendor's models at per-token prices, which makes it the natural
route for a metered seat or a cheap model in a loop. Two things to govern:

- **Set a spend cap on every key.** OpenRouter supports a per-key credit limit, and
  `GET https://openrouter.ai/api/v1/key` reports what remains. A cheap model in an agent
  loop still runs all night.
- **Read provider prices as provider prices.** An OpenRouter price is what one routed
  provider charges on that date, sometimes far under the vendor's list, and it can end
  without notice.

## nodeterm

A live canvas for terminals and agent sessions: one frame per project, agents side by side,
a "needs you" badge when an agent waits on a human. It is licensed under the Business
Source License 1.1, so treat it as optional tooling, never a dependency a policy or a
script requires.

## cc-switch

A desktop app (MIT) that shows what the vendor CLIs a person runs have used. Usage and
cost come from the CLIs' own local session logs. Remaining quota and account balances come
from its per-provider usage queries instead, and some providers' queries must be
configured before they report anything. Use it for that view. Keep the rest of it off:

- **Routing, failover, and aggregation off.** These modes put a local proxy between the
  agent and the vendor. The usage view works without them.
- **Provider management off for Codex.** Switching Codex providers in cc-switch migrates
  Codex history and writes Codex CLI config files, which changes a seat or lead's configuration
  outside anything that verified it.
- **Prompt, MCP, and skills sync off for any file `esc` or Balancewheel manages.** Two
  tools writing one file means one of them is always reporting drift it did not cause.

## OS-level containment: nono and `sandbox-exec`

A CLI's own permission settings are configuration, not enforcement. Enforcement is the
kernel refusing a write. Two backends do this on macOS:

- **nono** (Apache-2.0, `brew install nono`) allow-lists reads as well as writes. Its
  default profile denies credentials, keychains, browser data, shell history, and shell
  config files, so an agent under it cannot read `~/.ssh` even if talked into it. It also
  supports Linux.
- **`sandbox-exec`** (Seatbelt, built into macOS) needs no install. A generated Seatbelt policy
  denies writes everywhere except the runtime's state and temp directories and, for a
  lead, the worktree and the repository's common git dir, where a linked worktree's
  index, objects, and refs live. The common git dir's `hooks` and `config` stay denied.
  Reads stay open. Apple's man page marks it deprecated.

Verify containment by attack, never by reading a config: ask the agent to write inside and
outside the allowed paths, on a fresh and a resumed session, and check the disk. On
2026-10-24 the Balancewheel author ran three such checks under each backend, all passing,
with the kernel's `EPERM` as the evidence:

| Backend | Run | Inside the worktree | Outside it, `.git/hooks`, `.git/config`, a symlink escape, a child process writing outside |
|---|---|---|---|
| `sandbox-exec` | Kimi Code 2.3.1 as a read-only seat, fresh and resumed | every write denied, `.git` byte-identical | all denied |
| `sandbox-exec` | Kimi Code 2.3.1 as a lead | write and commit succeed | all denied |
| `sandbox-exec` | pi 0.85.1 with `deepseek/deepseek-v4.1-flash` as a lead | write and commit succeed | all denied |
| nono | Kimi Code 2.3.1 as a read-only seat, fresh and resumed | every write denied, `.git` byte-identical | all denied |
| nono | Kimi Code 2.3.1 as a lead | write and commit succeed | all denied |
| nono | pi 0.85.1 with `deepseek/deepseek-v4.1-flash` as a lead | write and commit succeed | all denied |

The same session found that Kimi Code 2.3.1 ignored its own `[[permission.rules]]` deny
rules in `-p` mode, and that its hooks fail open by its own documentation. That is why
the `kimi-models` and `model-seats` packs require an OS sandbox for unattended Kimi Code.

## ACP, per runtime

The [Agent Client Protocol](https://agentclientprotocol.com) lets an editor or a canvas
drive any agent through one interface. Support differs per runtime, and an adapter is an
extra install with its own version to track:

| Runtime | ACP support |
|---|---|
| Kimi Code | native: `kimi acp` |
| Codex CLI | through an adapter, `codex-acp` |
| pi | through an adapter, `pi-acp` |
| Claude Code | through an adapter built on the Claude Agent SDK (`claude-agent-acp`) |
| agy | listed in the ACP registry as Google Antigravity |

## Sources

- https://agentclientprotocol.com/get-started/agents and https://agentclientprotocol.com/get-started/registry (adapters and registry entries, retrieved 2026-10-24)
- https://github.com/MoonshotAI/kimi-code/blob/main/docs/en/reference/kimi-acp.md, `kimi-command.md`, and `customization/hooks.md` (retrieved 2026-10-24)
- https://openrouter.ai/docs/api/reference/limits (per-key credit limits, `GET /api/v1/key`; retrieved 2026-10-24)
- https://github.com/nolabs-ai/nono (license, Homebrew install, platforms; retrieved 2026-10-24)
- https://github.com/eneskirca/nodeterm (Business Source License 1.1; retrieved 2026-10-24)
- https://github.com/farion1231/cc-switch (MIT; retrieved 2026-10-24)
- https://github.com/earendil-works/pi (MIT; retrieved 2026-10-24)
- [Balancewheel](https://github.com/tensorgroup/balancewheel) design, `docs/superpowers/specs/2026-10-24-profiles-lead-rotation-cockpit-design.md`, "Profiles, lead rotation, a starter, and the cockpit" (2026-10-24): the containment runs, the Kimi Code deny-rule finding, agy's working-directory write, pi's stdin wait, and the cc-switch settings, all first-party observations.
