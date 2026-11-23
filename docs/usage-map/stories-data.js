// stories-data.js — the Why and Who tabs (hand-written). Strict JSON after the '=': no comments,
// no trailing commas, double quotes. The generator checks every reference: go run ./docs/usage-map/gen
window.USAGE_MAP_STORIES = {
 "why": {
  "lead": "OpenEscapement turns an organization's AI policy into a signed rule pack. Every repo pins a version, and esc writes that policy into the files coding agents already read. Teams stay in step, and a central team can see what each team uses.",
  "truthNote": "Each item has one source of truth. The pack repo holds the policy. Each repo holds the version it asks for. The portal only shows what repos chose to report.",
  "cards": [
   {
    "id": "models",
    "icon": "🧭",
    "title": "The right model, without being an expert",
    "text": "Model strengths, weaknesses and costs change every month. A curated guidance registry and the pack's model catalog say which model to use for what, and why. Developers get that advice in the files their agent reads, and do not need to follow every release.",
    "stories": [
     "mira-models",
     "ana-first-day"
    ]
   },
   {
    "id": "visibility",
    "icon": "🔭",
    "title": "See what every team uses",
    "text": "A central person or team sees which repos follow which policy version, and where files drift. They can use that view to help teams, and, if they choose, to govern. Reporting is opt-in and redacted.",
    "stories": [
     "lena-org-view",
     "mira-portal",
     "mira-reporting"
    ]
   },
   {
    "id": "sync",
    "icon": "🔗",
    "title": "Teams stay in sync",
    "text": "Every repo pins the same signed pack. A new version reaches a team as a normal pull request, and CI fails when files drift. So the development environment is the same in each team, and a new developer starts with it on day one.",
    "stories": [
     "kai-update",
     "kai-onboard",
     "theo-drift",
     "theo-ci"
    ]
   },
   {
    "id": "trust",
    "icon": "🔏",
    "title": "Policy you can trust",
    "text": "Packs are git repos with signed tags. esc verifies the signature before it writes, and records every hash in a lockfile. Nothing changes in a repo without a reviewable diff.",
    "stories": [
     "raj-author",
     "theo-sync"
    ]
   },
   {
    "id": "bw",
    "icon": "⚖️",
    "title": "Works with Balancewheel",
    "text": "Balancewheel finds out, from logged review panels, which models work best together and at what cost. OpenEscapement ships that seat policy to every team as a pack.",
    "stories": [
     "theo-balancewheel"
    ],
    "href": "https://tensorgroup.github.io/balancewheel/usage-map/#who",
    "linkLabel": "Open the Balancewheel map →"
   }
  ]
 },
 "org": {
  "name": "Windup Workshop",
  "blurb": "Windup Workshop is a fictional software company with four teams. Platform owns the rule pack. Payments and Mobile build products in repos that pin it. Leadership looks across all of them. The same people appear in the Balancewheel map.",
  "teams": [
   {
    "id": "platform",
    "name": "Platform",
    "about": "Owns the org's rule pack and runs the policy portal."
   },
   {
    "id": "payments",
    "name": "Payments",
    "about": "Builds the payments service. Its repo pins the org pack."
   },
   {
    "id": "mobile",
    "name": "Mobile",
    "about": "Builds the mobile app. Its repo pins the same pack."
   },
   {
    "id": "leadership",
    "name": "Leadership",
    "about": "Looks across every team."
   }
  ],
  "people": [
   {
    "id": "mira",
    "name": "Mira",
    "team": "platform",
    "role": "Platform admin",
    "goal": "Publish policy changes safely and see the whole fleet.",
    "personas": [
     "p-admin"
    ],
    "stories": [
     "mira-portal",
     "mira-models",
     "mira-reporting"
    ]
   },
   {
    "id": "raj",
    "name": "Raj",
    "team": "platform",
    "role": "Pack author",
    "goal": "Release a signed pack that every repo can pin and verify.",
    "personas": [
     "p-author"
    ],
    "stories": [
     "raj-author"
    ]
   },
   {
    "id": "theo",
    "name": "Theo",
    "team": "payments",
    "role": "Developer",
    "goal": "Keep the Payments repo on the org policy with no surprises.",
    "personas": [
     "p-dev",
     "p-ci",
     "p-bw"
    ],
    "stories": [
     "theo-sync",
     "theo-drift",
     "theo-ci",
     "theo-balancewheel"
    ]
   },
   {
    "id": "ana",
    "name": "Ana",
    "team": "payments",
    "role": "New developer",
    "goal": "Start work on day one with the same rules and models as her team.",
    "personas": [
     "p-dev"
    ],
    "stories": [
     "ana-first-day"
    ]
   },
   {
    "id": "kai",
    "name": "Kai",
    "team": "mobile",
    "role": "Developer",
    "goal": "Use the same policy as Payments, without extra work.",
    "personas": [
     "p-dev"
    ],
    "stories": [
     "kai-onboard",
     "kai-update"
    ]
   },
   {
    "id": "lena",
    "name": "Lena",
    "team": "leadership",
    "role": "Org observer",
    "goal": "Know what every team uses, how current it is, and later what it costs.",
    "personas": [
     "p-auditor"
    ],
    "stories": [
     "lena-org-view"
    ]
   }
  ]
 },
 "places": [
  {
   "id": "pack-repo",
   "title": "Org pack repo",
   "kind": "repo",
   "owner": "platform",
   "node": "packrepo",
   "truthFor": [
    "the org's rules, skills and model catalog",
    "each released version (a signed tag)"
   ]
  },
  {
   "id": "raj-laptop",
   "title": "Raj's terminal",
   "kind": "machine",
   "owner": "raj",
   "node": "p-author"
  },
  {
   "id": "portal",
   "title": "Policy portal (esc serve)",
   "kind": "service",
   "owner": "platform",
   "node": "portal",
   "truthFor": [
    "the reports repos chose to send (events.jsonl)"
   ]
  },
  {
   "id": "registry",
   "title": "Model guidance registry",
   "kind": "service",
   "node": "models",
   "truthFor": [
    "curated model facts: tier, roles, status, verified date"
   ]
  },
  {
   "id": "payments-repo",
   "title": "Payments repo",
   "kind": "repo",
   "owner": "payments",
   "node": "repo",
   "truthFor": [
    "the pack version Payments asks for (.escapement/config.yaml)",
    "what that pin resolved to (escapement.lock)"
   ]
  },
  {
   "id": "mobile-repo",
   "title": "Mobile repo",
   "kind": "repo",
   "owner": "mobile",
   "node": "repo",
   "truthFor": [
    "the pack version Mobile asks for (.escapement/config.yaml)"
   ]
  },
  {
   "id": "theo-laptop",
   "title": "Theo's terminal",
   "kind": "machine",
   "owner": "theo",
   "node": "p-dev"
  },
  {
   "id": "ana-laptop",
   "title": "Ana's terminal",
   "kind": "machine",
   "owner": "ana",
   "node": "p-dev"
  },
  {
   "id": "kai-laptop",
   "title": "Kai's terminal",
   "kind": "machine",
   "owner": "kai",
   "node": "p-dev"
  },
  {
   "id": "pack-cache",
   "title": "Pack cache",
   "kind": "home",
   "node": "gr-cache"
  },
  {
   "id": "ci",
   "title": "CI (GitHub Actions)",
   "kind": "service",
   "node": "ci"
  },
  {
   "id": "agent",
   "title": "Coding agent",
   "kind": "agent",
   "node": "p-agent"
  },
  {
   "id": "releases",
   "title": "esc releases",
   "kind": "service",
   "node": "dist"
  },
  {
   "id": "bw-home",
   "title": "Theo's Balancewheel log",
   "kind": "home",
   "owner": "theo",
   "node": "p-bw",
   "truthFor": [
    "Theo's panel results (his home directory)"
   ]
  },
  {
   "id": "lena-browser",
   "title": "Lena's browser",
   "kind": "machine",
   "owner": "lena",
   "node": "p-auditor"
  },
  {
   "id": "siem",
   "title": "Security team's SIEM",
   "kind": "service",
   "node": "pl-siem",
   "status": "planned"
  }
 ],
 "stories": [
  {
   "id": "ana-first-day",
   "person": "ana",
   "title": "First day on the team",
   "why": "The Payments repo is already under policy, so Ana's agent follows the org's rules and model guidance from her first session.",
   "steps": [
    {
     "place": "ana-laptop",
     "label": "Install esc",
     "text": "The installer downloads the release archive for her platform.",
     "command": "curl -sSfL https://raw.githubusercontent.com/tensorgroup/openescapement/main/install.sh | bash",
     "node": "d-install",
     "moves": [
      {
       "from": "releases",
       "to": "ana-laptop",
       "what": "esc archive and checksums",
       "kind": "read"
      }
     ]
    },
    {
     "place": "ana-laptop",
     "label": "Verify it",
     "text": "The installer checks the cosign signature on checksums.txt, then the archive's sha256, before it puts esc on the PATH. Without cosign it stops, unless ESC_SKIP_VERIFY=1 skips the signature check (not recommended). The sha256 check always runs.",
     "node": "d-sign",
     "moves": [
      {
       "from": "releases",
       "to": "ana-laptop",
       "what": "checksums.txt and its signature bundle",
       "kind": "read"
      }
     ]
    },
    {
     "place": "ana-laptop",
     "label": "Check by hand (optional)",
     "text": "VERIFYING.md gives the cosign, sha256 and SLSA provenance commands.",
     "command": "esc version",
     "node": "d-verify"
    },
    {
     "place": "payments-repo",
     "label": "Clone the repo",
     "text": "The rendered policy files, .escapement/config.yaml and the lockfile are committed, so a clone has them.",
     "node": "gr-blocks",
     "moves": [
      {
       "from": "payments-repo",
       "to": "ana-laptop",
       "what": "code, rendered policy files, config and lock",
       "kind": "read"
      }
     ],
     "truth": "payments-repo"
    },
    {
     "place": "agent",
     "label": "The agent reads the rules",
     "text": "Her coding agent reads CLAUDE.md, AGENTS.md or GEMINI.md, the skills and .mcp.json, as it does every session.",
     "node": "p-agent",
     "moves": [
      {
       "from": "ana-laptop",
       "to": "agent",
       "what": "managed blocks: rules and model guidance",
       "kind": "read"
      }
     ]
    },
    {
     "place": "ana-laptop",
     "label": "Confirm it is current",
     "text": "esc status compares each file with its hash in the lockfile.",
     "command": "esc status",
     "node": "cmd:status",
     "moves": [
      {
       "from": "payments-repo",
       "to": "ana-laptop",
       "what": "hashes from escapement.lock",
       "kind": "read"
      }
     ]
    }
   ]
  },
  {
   "id": "kai-onboard",
   "person": "kai",
   "title": "Bring a repo under policy",
   "why": "Kai puts the Mobile repo on the same pack as Payments. Nothing is written until a person says yes.",
   "steps": [
    {
     "place": "agent",
     "label": "Setup prompt (optional)",
     "text": "Kai can paste the README's setup prompt into his agent. It reads the existing instruction files and lists what a pack would overlap with.",
     "node": "p-agent",
     "moves": [
      {
       "from": "mobile-repo",
       "to": "agent",
       "what": "existing instruction files",
       "kind": "read"
      }
     ]
    },
    {
     "place": "mobile-repo",
     "label": "esc init",
     "text": "init scans for existing instruction files and writes .escapement/config.yaml. On a terminal it offers once to place the block marker; --yes declines and writes nothing else.",
     "command": "esc init",
     "node": "cmd:init",
     "moves": [
      {
       "from": "kai-laptop",
       "to": "mobile-repo",
       "what": "config.yaml",
       "kind": "store"
      }
     ]
    },
    {
     "place": "mobile-repo",
     "label": "Pin the pack",
     "text": "Kai points packs at the org pack repo: a git source, a ref, and a trusted signer in allowed_signers.",
     "node": "gr-config",
     "moves": [
      {
       "from": "kai-laptop",
       "to": "mobile-repo",
       "what": "pack source, ref and trusted signer",
       "kind": "store"
      }
     ],
     "truth": "mobile-repo"
    },
    {
     "place": "kai-laptop",
     "label": "Preview",
     "text": "Kai previews what will land, without writing.",
     "command": "esc render --stdout",
     "node": "cmd:render",
     "moves": [
      {
       "from": "pack-repo",
       "to": "kai-laptop",
       "what": "the pinned pack, rendered in memory",
       "kind": "read"
      }
     ]
    },
    {
     "place": "mobile-repo",
     "label": "Sync and commit",
     "text": "Kai writes the policy, then commits .escapement/, the rendered files and the lockfile on a branch.",
     "command": "esc sync",
     "node": "cmd:sync",
     "moves": [
      {
       "from": "pack-repo",
       "to": "mobile-repo",
       "what": "verified policy files",
       "kind": "sign"
      }
     ],
     "truth": "pack-repo"
    }
   ]
  },
  {
   "id": "theo-sync",
   "person": "theo",
   "title": "What esc sync does",
   "why": "One sync, step by step. Nothing is written after a verification or constraint failure.",
   "steps": [
    {
     "place": "theo-laptop",
     "label": "Run sync",
     "text": "Theo runs sync in the Payments repo.",
     "command": "esc sync",
     "node": "cmd:sync"
    },
    {
     "place": "payments-repo",
     "label": "Read the pin",
     "text": "esc reads the pinned packs from config.yaml.",
     "node": "gr-config",
     "moves": [
      {
       "from": "payments-repo",
       "to": "theo-laptop",
       "what": "pack source and ref",
       "kind": "read"
      }
     ],
     "truth": "payments-repo"
    },
    {
     "place": "pack-cache",
     "label": "Fetch",
     "text": "esc fetches each pinned ref with the system git, into the pack cache.",
     "node": "s-fetch",
     "moves": [
      {
       "from": "pack-repo",
       "to": "pack-cache",
       "what": "the pinned ref",
       "kind": "store"
      }
     ]
    },
    {
     "place": "pack-cache",
     "label": "Verify",
     "text": "esc verifies the tag signature. A bad signature or a moved tag stops here with exit 3.",
     "node": "s-verify",
     "moves": [
      {
       "from": "pack-repo",
       "to": "pack-cache",
       "what": "tag signature",
       "kind": "sign"
      }
     ],
     "truth": "pack-repo"
    },
    {
     "place": "pack-cache",
     "label": "Parse",
     "text": "esc loads pack.yaml, the rule fragments, skills, catalog and constraints.",
     "node": "s-parse"
    },
    {
     "place": "theo-laptop",
     "label": "Render",
     "text": "esc composes and renders each target: managed blocks, GOVERNANCE.md, skills and the .mcp.json merge.",
     "node": "s-render",
     "moves": [
      {
       "from": "pack-cache",
       "to": "theo-laptop",
       "what": "the pack's content",
       "kind": "read"
      }
     ]
    },
    {
     "place": "theo-laptop",
     "label": "Validate",
     "text": "esc checks the pack constraints. A finding is exit 1 and nothing is written.",
     "node": "s-validate"
    },
    {
     "place": "payments-repo",
     "label": "Write",
     "text": "esc writes atomically. A managed region someone edited by hand is skipped with a warning.",
     "node": "s-write",
     "moves": [
      {
       "from": "theo-laptop",
       "to": "payments-repo",
       "what": "rendered policy files",
       "kind": "store"
      }
     ]
    },
    {
     "place": "payments-repo",
     "label": "Lock",
     "text": "esc records every commit and hash in escapement.lock.",
     "node": "s-lock",
     "moves": [
      {
       "from": "theo-laptop",
       "to": "payments-repo",
       "what": "commits and hashes",
       "kind": "store"
      }
     ],
     "truth": "payments-repo"
    },
    {
     "place": "agent",
     "label": "Agents read it",
     "text": "The next agent session reads the new rules.",
     "node": "gr-blocks",
     "moves": [
      {
       "from": "payments-repo",
       "to": "agent",
       "what": "managed blocks",
       "kind": "read"
      }
     ]
    },
    {
     "place": "portal",
     "label": "Report (opt-in)",
     "text": "If the pack asks for reporting and a token is set, a redacted report goes to the portal. This never changes the exit code.",
     "node": "s-report",
     "moves": [
      {
       "from": "theo-laptop",
       "to": "portal",
       "what": "redacted report",
       "kind": "share"
      }
     ]
    }
   ]
  },
  {
   "id": "theo-drift",
   "person": "theo",
   "title": "Check for drift",
   "why": "Status has two separate axes: managed content esc owns, and local content it does not. A local addition alone is never drift.",
   "steps": [
    {
     "place": "theo-laptop",
     "label": "Status",
     "text": "See the state of every artifact.",
     "command": "esc status",
     "node": "cmd:status",
     "moves": [
      {
       "from": "payments-repo",
       "to": "theo-laptop",
       "what": "files and lockfile hashes",
       "kind": "read"
      }
     ]
    },
    {
     "place": "payments-repo",
     "label": "Compare",
     "text": "status compares each artifact on disk with its hash in the lockfile: in-sync, altered, stale, missing, orphan or occupied.",
     "node": "gr-lock",
     "truth": "payments-repo"
    },
    {
     "place": "theo-laptop",
     "label": "Diff",
     "text": "See the drift as a diff.",
     "command": "esc diff",
     "node": "cmd:diff"
    },
    {
     "place": "payments-repo",
     "label": "Fix the edit",
     "text": "A hand edit inside a managed block is drift. Move the change outside the block, or propose it to the pack.",
     "node": "gr-blocks",
     "moves": [
      {
       "from": "theo-laptop",
       "to": "pack-repo",
       "what": "a proposed rule change",
       "kind": "share"
      }
     ],
     "truth": "pack-repo"
    },
    {
     "place": "payments-repo",
     "label": "Overwrite",
     "text": "Overwrite the managed region. --force never touches a local addition.",
     "command": "esc sync --force",
     "node": "cmd:sync",
     "moves": [
      {
       "from": "pack-cache",
       "to": "payments-repo",
       "what": "managed region from the pinned pack",
       "kind": "store"
      }
     ]
    }
   ]
  },
  {
   "id": "theo-ci",
   "person": "theo",
   "title": "Gate pull requests in CI",
   "why": "One Action step makes drift visible on every pull request, in every team that adds it.",
   "steps": [
    {
     "place": "payments-repo",
     "label": "Add the Action",
     "text": "Add the Action to the repo's workflow once the pack is a git source.",
     "command": "- uses: tensorgroup/openescapement@main",
     "node": "ci-action",
     "moves": [
      {
       "from": "theo-laptop",
       "to": "payments-repo",
       "what": "workflow step",
       "kind": "store"
      }
     ]
    },
    {
     "place": "ci",
     "label": "Run the gate",
     "text": "The Action installs esc with go install and runs the check.",
     "command": "esc status --check",
     "node": "ci-gate",
     "moves": [
      {
       "from": "payments-repo",
       "to": "ci",
       "what": "files and lockfile",
       "kind": "read"
      }
     ]
    },
    {
     "place": "ci",
     "label": "Fail on drift",
     "text": "Drift or a constraint finding exits 1 and fails the job. Integrity problems are exit 3.",
     "node": "exit:1"
    },
    {
     "place": "payments-repo",
     "label": "Block the merge",
     "text": "When branch protection requires this check, the pull request cannot merge until the policy files match the pinned pack again.",
     "node": "ci-pr",
     "moves": [
      {
       "from": "ci",
       "to": "payments-repo",
       "what": "failed check",
       "kind": "share"
      }
     ],
     "truth": "payments-repo"
    }
   ]
  },
  {
   "id": "kai-update",
   "person": "kai",
   "title": "Move to a new policy version",
   "why": "Sync never jumps versions by itself. When Mobile moves to the version Payments uses, the two teams match again, through a normal pull request.",
   "steps": [
    {
     "place": "pack-repo",
     "label": "A new tag",
     "text": "The pack author signs a new tag.",
     "node": "pr-tag",
     "truth": "pack-repo"
    },
    {
     "place": "mobile-repo",
     "label": "Notice it",
     "text": "If the pack declares update_check, esc notices the newer tag and logs it in update-log.jsonl.",
     "node": "gr-updatelog",
     "moves": [
      {
       "from": "pack-repo",
       "to": "mobile-repo",
       "what": "newer tag noticed",
       "kind": "store"
      }
     ]
    },
    {
     "place": "kai-laptop",
     "label": "Preview the change",
     "text": "See what the new version would change.",
     "command": "esc diff --against v1.1.0",
     "node": "cmd:diff",
     "moves": [
      {
       "from": "pack-repo",
       "to": "kai-laptop",
       "what": "the new version",
       "kind": "read"
      }
     ]
    },
    {
     "place": "mobile-repo",
     "label": "Bump the pin",
     "text": "Bump the pin in config and lock.",
     "command": "esc update --ref v1.1.0",
     "node": "cmd:update",
     "moves": [
      {
       "from": "kai-laptop",
       "to": "mobile-repo",
       "what": "new ref",
       "kind": "store"
      }
     ],
     "truth": "mobile-repo"
    },
    {
     "place": "mobile-repo",
     "label": "Apply it",
     "text": "Write the new policy.",
     "command": "esc sync",
     "node": "cmd:sync",
     "moves": [
      {
       "from": "pack-repo",
       "to": "mobile-repo",
       "what": "verified policy files",
       "kind": "sign"
      }
     ]
    },
    {
     "place": "mobile-repo",
     "label": "Review",
     "text": "Open a pull request. The policy change is a normal, reviewable diff.",
     "node": "ci-pr",
     "moves": [
      {
       "from": "kai-laptop",
       "to": "mobile-repo",
       "what": "pull request",
       "kind": "share"
      }
     ]
    }
   ]
  },
  {
   "id": "raj-author",
   "person": "raj",
   "title": "Write and release a pack",
   "why": "The org's signature must cover the exact bytes an agent will run, so external skills are vendored, not referenced.",
   "steps": [
    {
     "place": "raj-laptop",
     "label": "Start from an example",
     "text": "Start from an example pack: acme-org for a baseline, or a model pack.",
     "node": "packs"
    },
    {
     "place": "pack-repo",
     "label": "Edit pack.yaml",
     "text": "Edit pack.yaml: rules, skills, MCP servers, catalog, constraints, and optionally update_check and reporting.",
     "node": "pr-manifest",
     "moves": [
      {
       "from": "raj-laptop",
       "to": "pack-repo",
       "what": "pack.yaml",
       "kind": "store"
      }
     ],
     "truth": "pack-repo"
    },
    {
     "place": "pack-repo",
     "label": "Write rules",
     "text": "Write rule fragments in the committed style (docs/pack-authoring.md).",
     "node": "pr-rules",
     "moves": [
      {
       "from": "raj-laptop",
       "to": "pack-repo",
       "what": "rule fragments",
       "kind": "store"
      }
     ]
    },
    {
     "place": "raj-laptop",
     "label": "Vendor a skill",
     "text": "Copy an external skill into the pack, with its provenance.",
     "command": "esc pack add-skill https://github.com/org/skills#subdir --ref v1.2.0",
     "node": "cmd:pack-add-skill"
    },
    {
     "place": "pack-repo",
     "label": "Record provenance",
     "text": "The skill is copied byte-identical; sources.yaml records where it came from.",
     "node": "pr-skills",
     "moves": [
      {
       "from": "raj-laptop",
       "to": "pack-repo",
       "what": "skill files and sources.yaml",
       "kind": "store"
      }
     ]
    },
    {
     "place": "raj-laptop",
     "label": "Check freshness",
     "text": "Check whether vendored skills are behind upstream; --check exits 1 for CI.",
     "command": "esc pack outdated --check",
     "node": "cmd:pack-outdated",
     "moves": [
      {
       "from": "pack-repo",
       "to": "raj-laptop",
       "what": "vendored skill versions",
       "kind": "read"
      }
     ]
    },
    {
     "place": "pack-repo",
     "label": "Sign a release",
     "text": "Release with a signed tag. Repos get it only when they update and sync.",
     "command": "git tag -s v1.1.0 && git push --tags",
     "node": "pr-tag",
     "moves": [
      {
       "from": "raj-laptop",
       "to": "pack-repo",
       "what": "signed tag v1.1.0",
       "kind": "sign"
      }
     ],
     "truth": "pack-repo"
    }
   ]
  },
  {
   "id": "mira-portal",
   "person": "mira",
   "title": "See the fleet and publish",
   "why": "The portal turns many repos into one table and can publish a pack edit. It never pushes rules into a repo.",
   "steps": [
    {
     "place": "portal",
     "label": "Start the portal",
     "text": "Start the portal. --demo seeds a fictional university to explore.",
     "command": "esc serve --demo",
     "node": "cmd:serve"
    },
    {
     "place": "portal",
     "label": "Fleet page",
     "text": "The fleet page shows each repo's policy state, from the reports repos chose to send.",
     "node": "route:GET /fleet",
     "truth": "portal"
    },
    {
     "place": "portal",
     "label": "Edit a rule",
     "text": "Edit a pack's rule fragment in the browser.",
     "node": "route:GET /packs/{name}/edit"
    },
    {
     "place": "pack-repo",
     "label": "Publish",
     "text": "Publish validates, commits and tags in a server-side clone. Nothing is half-published. The portal's tag is annotated, not signed.",
     "node": "route:POST /packs/{name}/publish",
     "moves": [
      {
       "from": "portal",
       "to": "pack-repo",
       "what": "commit and annotated tag",
       "kind": "share"
      }
     ],
     "truth": "pack-repo"
    },
    {
     "place": "pack-repo",
     "label": "Sign a release",
     "text": "Repos that trust only signed tags need a signed release. Raj signs a tag on that commit.",
     "command": "git tag -s v1.2.0 && git push --tags",
     "node": "pr-tag",
     "moves": [
      {
       "from": "raj-laptop",
       "to": "pack-repo",
       "what": "signed tag",
       "kind": "sign"
      }
     ],
     "truth": "pack-repo"
    },
    {
     "place": "payments-repo",
     "label": "Reach a repo",
     "text": "The change reaches a repo only when someone updates and syncs there.",
     "node": "cmd:sync",
     "moves": [
      {
       "from": "pack-repo",
       "to": "payments-repo",
       "what": "the new version, after update and sync",
       "kind": "sign"
      }
     ]
    }
   ]
  },
  {
   "id": "mira-reporting",
   "person": "mira",
   "title": "Turn on reporting",
   "why": "Reporting is opt-in, redacted by level, and never breaks a sync. No prompt content is collected.",
   "steps": [
    {
     "place": "pack-repo",
     "label": "Declare reporting",
     "text": "The pack declares reporting with an amendments level (off, metrics or content) and an endpoint. A repo may only lower it.",
     "node": "pr-manifest",
     "truth": "pack-repo"
    },
    {
     "place": "payments-repo",
     "label": "Set the token",
     "text": "The repo sets ESC_PORTAL_TOKEN in its environment. Without it, esc prints one warning and sends nothing.",
     "node": "gr-config"
    },
    {
     "place": "payments-repo",
     "label": "Send a report",
     "text": "After sync or status, esc redacts the report to the level and sends it.",
     "node": "s-report",
     "moves": [
      {
       "from": "payments-repo",
       "to": "portal",
       "what": "redacted report",
       "kind": "share"
      }
     ]
    },
    {
     "place": "portal",
     "label": "Store it",
     "text": "The portal stores the event in events.jsonl.",
     "node": "route:POST /api/v1/events",
     "moves": [
      {
       "from": "payments-repo",
       "to": "portal",
       "what": "event",
       "kind": "store"
      }
     ],
     "truth": "portal"
    },
    {
     "place": "payments-repo",
     "label": "Queue if down",
     "text": "If the portal is down, the report waits in the outbox (at most 200 entries, 14 days) and is sent next time.",
     "node": "gr-outbox",
     "moves": [
      {
       "from": "payments-repo",
       "to": "payments-repo",
       "what": "outbox.jsonl (gitignored)",
       "kind": "store"
      }
     ]
    },
    {
     "place": "portal",
     "label": "Usage pages",
     "text": "Usage and fleet pages read the stored events.",
     "node": "route:GET /usage"
    }
   ]
  },
  {
   "id": "mira-models",
   "person": "mira",
   "title": "Model guidance reaches every team",
   "why": "Which model, for what, and why: curated once in the registry, shipped through the pack's catalog, and read by every agent.",
   "steps": [
    {
     "place": "registry",
     "label": "The registry",
     "text": "The registry lists each vendor's models with tier, roles, status and the date each fact was verified.",
     "node": "models",
     "truth": "registry"
    },
    {
     "place": "portal",
     "label": "Read it in the portal",
     "text": "The models pages show the registry in the portal.",
     "node": "route:GET /models",
     "moves": [
      {
       "from": "registry",
       "to": "portal",
       "what": "model facts",
       "kind": "read"
      }
     ]
    },
    {
     "place": "portal",
     "label": "Adopt a starter",
     "text": "Mira adopts a vendor's starter fragment into a pack.",
     "node": "route:GET /models/{vendor}/adopt",
     "moves": [
      {
       "from": "portal",
       "to": "pack-repo",
       "what": "starter fragment",
       "kind": "share"
      }
     ]
    },
    {
     "place": "pack-repo",
     "label": "The catalog",
     "text": "A model pack carries a catalog: each model is preferred, reserved, allowed, review-required or banned, with the reason.",
     "node": "pack:anthropic-models",
     "truth": "pack-repo"
    },
    {
     "place": "payments-repo",
     "label": "Rendered everywhere",
     "text": "After a signed release, the catalog renders into the managed blocks and GOVERNANCE.md of every repo that pins the pack and syncs.",
     "node": "pr-catalog",
     "moves": [
      {
       "from": "pack-repo",
       "to": "payments-repo",
       "what": "model catalog",
       "kind": "sign"
      },
      {
       "from": "pack-repo",
       "to": "mobile-repo",
       "what": "model catalog",
       "kind": "sign"
      }
     ]
    },
    {
     "place": "agent",
     "label": "Agents follow it",
     "text": "Each team's agent reads the same model guidance in its managed block.",
     "node": "gr-blocks",
     "moves": [
      {
       "from": "payments-repo",
       "to": "agent",
       "what": "which model for what",
       "kind": "read"
      }
     ]
    }
   ]
  },
  {
   "id": "theo-balancewheel",
   "person": "theo",
   "title": "Ship a seat policy from Balancewheel",
   "why": "Balancewheel earns a seat policy from logged panel results. A pack is what that policy becomes for a team.",
   "steps": [
    {
     "place": "bw-home",
     "label": "Panel results",
     "text": "A Balancewheel panel logs which reviewer seat catches what, under Theo's home directory.",
     "node": "p-bw",
     "truth": "bw-home"
    },
    {
     "place": "pack-repo",
     "label": "Pack form",
     "text": "examples/packs/model-seats is that policy in pack form: seats named by role, models as a dated detail.",
     "node": "pack:model-seats",
     "moves": [
      {
       "from": "theo-laptop",
       "to": "pack-repo",
       "what": "seat policy, copied by hand",
       "kind": "share"
      }
     ]
    },
    {
     "place": "pack-repo",
     "label": "Sign it",
     "text": "The team copies it into its pack repo and signs a tag.",
     "node": "pr-tag",
     "truth": "pack-repo"
    },
    {
     "place": "payments-repo",
     "label": "Sync it",
     "text": "Each repo pins and syncs it like any other pack. Neither tool reads the other's state.",
     "node": "cmd:sync",
     "moves": [
      {
       "from": "pack-repo",
       "to": "payments-repo",
       "what": "signed seat policy",
       "kind": "sign"
      },
      {
       "from": "pack-repo",
       "to": "mobile-repo",
       "what": "signed seat policy",
       "kind": "sign"
      }
     ]
    }
   ]
  },
  {
   "id": "lena-org-view",
   "person": "lena",
   "title": "See what the whole org uses",
   "why": "Lena needs one view across teams. Part of it works today, from reports. The rest is planned.",
   "steps": [
    {
     "place": "lena-browser",
     "label": "Fleet table",
     "text": "The fleet page shows each repo's policy state, from the reports repos chose to send.",
     "node": "route:GET /fleet",
     "moves": [
      {
       "from": "portal",
       "to": "lena-browser",
       "what": "each repo's pack version and drift",
       "kind": "read"
      }
     ],
     "truth": "portal"
    },
    {
     "place": "lena-browser",
     "label": "Usage",
     "text": "The usage page reads the same stored events.",
     "node": "route:GET /usage",
     "moves": [
      {
       "from": "portal",
       "to": "lena-browser",
       "what": "stored events",
       "kind": "read"
      }
     ]
    },
    {
     "place": "lena-browser",
     "label": "esc fleet",
     "text": "Planned: esc fleet prints the fleet table in a terminal.",
     "command": "esc fleet --json",
     "node": "pl-fleet",
     "status": "planned"
    },
    {
     "place": "agent",
     "label": "Live policy",
     "text": "Planned: an MCP server gives agents live policy. The static files stay the floor when it is down.",
     "node": "pl-mcp",
     "status": "planned"
    },
    {
     "place": "portal",
     "label": "Registry",
     "text": "Planned: a self-service registry with expiry becomes the shadow-IT map.",
     "node": "pl-registry",
     "status": "planned"
    },
    {
     "place": "siem",
     "label": "SIEM export",
     "text": "Planned: governance events go to the security team's syslog or SIEM.",
     "node": "pl-siem",
     "moves": [
      {
       "from": "portal",
       "to": "siem",
       "what": "governance events",
       "kind": "share"
      }
     ],
     "status": "planned"
    },
    {
     "place": "portal",
     "label": "Reviews",
     "text": "Planned: exceptions and an audit trail become evidence for SOC 2 and ISO 42001.",
     "node": "pl-review",
     "status": "planned"
    }
   ]
  }
 ]
};
