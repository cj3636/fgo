# fGo — Alpha Project Roadmap (Git‑Lite File Server)

> Repo: `https://git.tyss.io/cj3636/fgo` (PRs welcome! Moving to github.com soon)
> Go: **1.24**
> Purpose: A simple, fast, and secure VCS File Server with Git-like versioning for personal and small team use.
> Deployment: homelab first, production-grade patterns included

- ***NOTE:*** This document uses fgo ⇄ git command names interchangeably.
  - The application code and documentation, and user interface should ONLY use the custom fgo terminology.
  - git terminology is only for user familiarity in help text and CLI aliases.

---

## Overview

A pragmatic, extensible roadmap that ships working software at every milestone. Focus is the **server first**, with a
minimal but stable wire protocol and a thin CLI later. Each stage is production‑usable within its stated limits.

**Design priorities:** speed → data integrity → security → simplicity. Everything is pluggable behind small Go interfaces.

---

## Naming

- `fgo` CLI client (Git‑lite; like `git`)
- `gofile` server (HTTP API)

---

## 0) Canon & Terminology

**Canonical (API / server) names**:

- **space** (top‑level partition)
- **box** (a versioned collection; like a repo)
- **branch** (mutable ref to a commit, default `main`)
- **commit** (immutable manifest of paths → blobs)
- **blob** (content‑addressed file by sha256)
- **visibility**: `public | unlisted | private`

***THE APPLICATION IS TO USE THE CUSTOM NAMES ONLY!***
*THE ONLY PLACE THE GIT NAMES SHOULD APPEAR IS IN HELP TEXT AND AS CLI ALIASES!!!*

**Client Commands** — exposed by `fgo` :

- space ⇄ **namespace** (user/account boundary)
- box ⇄ **repo** (versioned collection, repo)
- arm ⇄ **branch** (manage branches / refs)
- enact ⇄ **commit** ("actions" not commits - create, ammend, sign, etc. a commit. does not have to be actual file change: no-ops need enact ran as well.ie. settings update, todos, etc. Changes to repo settings or metadata are also actions that require running enact)
- dupe ⇄ **clone** (copy a box locally / identical to git clone)
- grab ⇄ **pull** (download new changes and apply to local manifest / files)
- call ⇄ **fetch** (fetch remote data without applying, like git fetch)
- place ⇄ **push** (upload local changes to remote box, like git push. requires at least one `enact` prior)
- replace ⇄ **force place** (force push to branch head, like git push --force. Use with caution. ALWAYS creates a new local backup and a 24hr remote backup even if backup settings are off or limited)
- store ⇄ **push to specific arm** (place to a specific branch, like git push origin <branch>)
- sign ⇄ **tag** (create a signed commit reference, like git tag)
- show ⇄ **show** (display metadata about a path / blob / commit)
- info ⇄ **status** (show local manifest status vs remote box)
- log ⇄ **log** (show commit history)
- init ⇄ **LOCAL box create -> remote** (create a new box locally without pushing to server or setting up remote tracking. like git init)
- new ⇄ **REMOTE box create -> local** (create a new box on remote server without setting up local tracking, like git remote add + git git clone.      Can be used to create a box on server without cloning locally. Templates for repo type, visibility, etc. can be specified. Defaults are to create: .fgo/ root with: README, ignores, config, LICENSE, etc.)
- Special / Multi-Op Tools - *interactive* commands that can operate on multiple boxes at once:
  - create ⇄ **REMOTE create, initialize as type / template, setup & config, clone** (create a new box on remote server and set up local tracking, like git init + *user creates & adds content*+ git add . + git remote add origin <url> + git push -u origin main). `fgo create <space.box:arm@version> from <local|template|language|etc.>` for predefined box types (eg. website, project, docs, etc.)
  - upload ⇄ **git commit -am ""** (add files to local manifest for next enact)

> Rule: protocol & API use these canonical terms. Help text maps my names ↔ canonical names *only* for user convenience.

**IDs:** ULID (sortable) for boxes/commits; blobs by sha256. JSON fields use `snake_case`.

---

## Current Instructions

- Continue working on the steps in [Roadmap](../docs/ROADMAP.md)
- Check off items as you complete them