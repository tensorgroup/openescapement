---
name: sweep-sources
description: Sweep the sources that shape instruction-file conventions and model guidance, verify every candidate against a primary source, log the result in docs/roadmap/vendor-guidance-tracking.md, and end with a packs-need-updating verdict. Use when asked to sweep sources, check vendor guidance, or when a weekly or monthly sweep is due. Takes weekly, deep, or all as its argument; with none it runs what is due.
---

# Sweep sources

The tracking doc `docs/roadmap/vendor-guidance-tracking.md` owns the source list, the cadence, and the log. This skill reads that file and never repeats its contents. When the two disagree, the tracking doc wins: follow it, and report the mismatch in your output so the skill gets fixed in its own change.

## 1. Decide which tier to run

Argument `$ARGUMENTS` is one of `weekly`, `deep`, `all`, or empty.

- `weekly`: the community-signal scan.
- `deep`: the primary-source vendor check. A deep run includes the weekly scan.
- `all`: same as `deep`.
- empty: compute what is due from the log.

Due-date rule. Take today's date from `date +%F`. Read the `## Log` section. Find the newest entry tagged `[weekly]` or `[deep]`; if it is 7 or more days old, or there is none, the weekly tier is due. Find the newest entry tagged `[deep]`; if it is 30 or more days old, or there is none, the deep tier is due. Entries with no tag are historical and do not count. If the deep tier is due, run `deep`. Else if the weekly tier is due, run `weekly`. Else say which dates were found, state that nothing is due, and stop.

Print one line before sweeping: the tier, today's date, and the last-run date per tier.

## 2. Sweep

Read the source lists from the tracking doc's `## Sources` section: `### Primary sources per vendor` and `### Community signal (weekly tier)`.

Filter every source to what this product cares about, and nothing else:

- instruction-file conventions: new files, renames, hierarchy changes, size-cap changes, import syntax
- context and prompting guidance changes from a vendor
- a new coding agent worth rendering to
- a major model-generation release, or a model retirement, that changes the model registry or a model pack

Ignore hype, benchmarks, pricing rumors, and product drama.

Weekly scan, per community source:

- Subreddits: fetch `https://old.reddit.com/r/<name>/top/?t=week`, then the `top.json?t=week` and `top.rss?t=week` forms of the same URL, then web search restricted to `site:reddit.com/r/<name>` for the past week. If every route fails, record the subreddit as unreachable in the log entry, not as no signal. A subreddit that returns "not found" or "banned" is a source-list correction, not a finding.
- Hacker News: query `https://hn.algolia.com/api/v1/search_by_date?tags=story&numericFilters=created_at_i>EPOCH&query=TERM`, where `EPOCH` is the Unix time seven days ago (`date -v-7d +%s` on macOS, `date -d '7 days ago' +%s` on Linux), once per term: `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `context engineering`, `Claude Code`, `Codex CLI`, `Gemini CLI`, `Kimi Code`, `Grok Build`.

Deep check, per primary source: fetch each URL listed under `### Primary sources per vendor` and read for changes since the newest `[deep]` entry (or since the newest entry for that vendor when there is no `[deep]` entry yet, or the vendor's full current page when it has never been logged). For vendors whose URL is a docs index, follow the instruction-file or customization page one level down.

Collect candidates as: source, claim, URL, date seen.

## 3. Verify

For every candidate from a community source, find the primary source that supports it and fetch it. Discard any candidate the primary source does not support, and say so in the log entry when the community claim was wrong (a wrong claim is itself useful signal about a source). Never log a claim on community evidence alone.

Note corrections to the source list: a dead or renamed subreddit, a moved docs URL. Apply the correction to the tracking doc's Sources section in the same edit as the log entry.

## 4. Log and assess

Append one entry at the top of `## Log` in this shape, newest first:

```
- **YYYY-MM-DD** — [weekly|deep] <source(s)>. <What changed, with the primary-source URL.> Action: <what was done, or "none">.
```

A deep entry may carry one indented sub-bullet per vendor, each ending with its own `Action:`, with the verdict line as the last sub-bullet; the 2026-09-16 entry is the precedent.

With nothing to report, tag the tier that actually ran, so its last-run date still advances:

```
- **YYYY-MM-DD** — [weekly] no signal.
- **YYYY-MM-DD** — [deep] no signal.
- **YYYY-MM-DD** — [weekly] community tier: reddit unreachable; Hacker News no signal.
```

Then assess impact against, in order:

1. the example packs under `examples/packs/` (rule text, catalogs, prices, model IDs)
2. `docs/pack-authoring.md` (does new vendor guidance change the style rule?)
3. renderer targets and defaults (`internal/targets/`, `internal/render/`)
4. the model registry (`internal/guidance/models/models.yaml` and the per-vendor notes beside it)

End the entry, and the session's output, with the verdict line:

```
packs need updating: yes|no. <reasons>
```

## Boundaries

- This skill edits exactly one file: the tracking doc (log entry, source-list corrections). It never patches a pack, the renderer, or the registry. A `yes` verdict is a request for the normal spec → design-doc cycle.
- A finding that changes render targets, renderer defaults, or the model registry is out of scope for this skill's edit and is flagged in the verdict for that cycle.
- Commit nothing. Leave the tracking-doc change in the working tree for review.
