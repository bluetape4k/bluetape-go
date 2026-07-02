# Issue #310 Libvips Evaluation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Evaluate libvips-backed Go image adapters and ship a non-default govips example only if the evidence justifies it.

**Architecture:** Keep the root module and `imagekit` package pure Go. Put native libvips code in `examples/imagekit-govips` as a nested optional module with its own tests, benchmark, and README. Record the decision in repo docs and preserve external source evidence in the bluetape4k wiki.

**Tech Stack:** Go 1.26, `imagekit`, `github.com/davidbyttow/govips/v2 v2.18.0`, native libvips 8.18.3.

---

### Task 1: Record Evaluation Evidence

**Files:**
- Create: `docs/research/2026-07-02-issue-310-libvips-evaluation.md`
- Create: `docs/superpowers/specs/2026-07-02-issue-310-libvips-evaluation-design.md`
- Create: `docs/superpowers/plans/2026-07-02-issue-310-libvips-evaluation-plan.md`

- [x] **Step 1: Collect candidate metadata**

Run:
```bash
gh repo view davidbyttow/govips --json nameWithOwner,isArchived,licenseInfo,stargazerCount,forkCount,pushedAt,updatedAt,latestRelease
gh repo view h2non/bimg --json nameWithOwner,isArchived,licenseInfo,stargazerCount,forkCount,pushedAt,updatedAt,latestRelease
go list -m -json github.com/davidbyttow/govips/v2@latest
go list -m -json github.com/h2non/bimg@latest
```

Expected: govips has a current v2 module; bimg/v2 is unavailable and bimg v1 is older.

- [x] **Step 2: Collect native and benchmark evidence**

Run:
```bash
vips --version
pkg-config --modversion vips
CGO_CFLAGS_ALLOW='-Xpreprocessor' go test -run '^$' -bench . -benchmem ./...
```

Expected: local libvips is detected and clean benchmark rows compare imagekit, govips, and bimg.

### Task 2: Add Test-First Optional Example Module

**Files:**
- Create: `examples/imagekit-govips/go.mod`
- Create: `examples/imagekit-govips/adapter_test.go`
- Create: `examples/imagekit-govips/adapter.go`
- Create: `examples/imagekit-govips/README.md`
- Create: `examples/imagekit-govips/README.ko.md`

- [ ] **Step 1: Write failing tests**

Tests must call `Transform`, `TransformTo`, `RuntimeInfo`, and `Startup` before implementation exists.

Run:
```bash
CGO_CFLAGS_ALLOW='-Xpreprocessor' go test ./...
```

Expected: FAIL because the adapter API is undefined.

- [ ] **Step 2: Implement minimal adapter**

Implement bounded read, context checks, govips startup, JPEG/PNG export, request mapping, image handle cleanup, and runtime metadata.

- [ ] **Step 3: Verify green tests and race**

Run:
```bash
CGO_CFLAGS_ALLOW='-Xpreprocessor' go test ./...
CGO_CFLAGS_ALLOW='-Xpreprocessor' go test -race ./...
CGO_CFLAGS_ALLOW='-Xpreprocessor' go test -run '^$' -bench . -benchmem ./...
```

Expected: tests pass and benchmark rows are generated.

### Task 3: Preserve Research and Validate Root Isolation

**Files:**
- Create: `/Users/debop/work/bluetape4k/bluetape4k-wiki/research/2026-07-02-issue-310-libvips-go-evaluation.md`
- Create: `docs/lessons/2026-07-02-issue-310-libvips-evaluation.md`

- [ ] **Step 1: Preserve external source summary**

Write a copyright-safe wiki research note summarizing govips and bimg metadata,
README setup requirements, runtime lifecycle notes, and benchmark implications.

- [ ] **Step 2: Verify root module stays pure-Go by default**

Run from the repository root:
```bash
go test ./...
git diff --check
```

Expected: root tests do not require compiling the nested libvips example module.

- [ ] **Step 3: Run review and create PR**

Run local 7-tier review, commit with Lore trailers, push, create a PR that closes
#310, and ensure the PR body ends with `## DoD Status`.
