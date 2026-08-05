# Seed refresh: guidance upgrades and always-fresh demo data

Status: approved design, 2026-08-04. Extends the Models guidance spec's §2 seeding behavior; motivated by a real defect: a guidance dir seeded before the starters shipped kept its old models.yaml forever (create-if-missing never refreshes), silently hiding the starter blocks and adopt buttons.

## 1. Guidance seeding: create-or-refresh-unmodified

`guidance.Seed` gains a hash manifest and three-way semantics per embedded file:

- The manifest lives at `<data-dir>/guidance/.seeded.json`: a map of rel path to the sha256 of the content that was last seeded there. It is machine-managed; hand edits to it are not supported.
- Absent on disk: write the embedded content and record its hash (unchanged from today).
- Present and disk hash equals the recorded seeded hash (the user never edited it): if the embedded content now differs, overwrite with the new embedded content and update the record. Unedited content stays current across upgrades.
- Present and disk hash differs from the recorded hash (user-edited): leave the file untouched, exactly as today. User edits are never overwritten.
- Files recorded in the manifest but no longer in the embedded tree: left on disk, record retained (no deletion semantics in v1).

Migration for pre-manifest dirs (no `.seeded.json`): compare each disk file against the current embedded content; matching files get recorded (provably unmodified); non-matching files are left alone and unrecorded, because old-seed-unmodified and user-edited are indistinguishable without history. Such files stay frozen until hand-refreshed; this one-time gap is accepted and covered in practice by the demo reset below. The manifest write itself is atomic (temp plus rename).

## 2. Demo mode: always-fresh example data

`esc serve --demo` resets the demo-owned data to pristine on every startup, before seeding: the org store seed data, the seeded pack repos directory, the demo governed repo, and the guidance directory are deleted and recreated. A single stdout line states that demo data was reset.

- Rationale: demos must be deterministic and always show current example content; the create-if-missing staleness class disappears entirely in demo mode.
- Accepted trade, stated in the README: restarting a demo server discards state created during the demo (published pack versions, adopted starters, guidance edits). Non-demo mode is unaffected and keeps the §1 refresh-unmodified semantics.
- Only demo-owned paths are removed; the reset never touches anything outside the server data dir.

## 3. Testing

- Seed manifest: three-way table (absent created and recorded; unmodified-and-changed refreshed with record updated; edited left alone), manifest atomicity, migration behavior for a pre-manifest dir with one matching and one non-matching file.
- Demo reset: a modified demo file (guidance note and a demo-repo file) is pristine again after a second demo startup; non-demo startup leaves edits in place; reset message printed.
- The defect scenario as a regression test: seed with an old-content models.yaml recorded in the manifest, upgrade the embedded content, re-seed, assert the disk registry gains the new fields.

## 4. Docs

README: the guidance-editing paragraph replaces the delete-to-re-seed caveat with the refresh-unmodified behavior; the demo section states demo data resets on every startup. CHANGELOG bullet. No em-dashes.
