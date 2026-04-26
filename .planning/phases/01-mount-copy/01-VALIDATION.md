---
phase: 1
slug: mount-copy
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-25
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | cli/go.mod |
| **Quick run command** | `cd cli && go test ./... -run TestMount -v` |
| **Full suite command** | `cd cli && go test ./...` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd cli && go test ./... -run TestMount -v`
- **After every plan wave:** Run `cd cli && go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 1-01-01 | 01 | 1 | MOUNT-01 | — | Read-only mount prevents container writes to host ~/.claude | unit | `cd cli && go test ./... -run TestClaudeMount` | ❌ W0 | ⬜ pending |
| 1-01-02 | 01 | 1 | MOUNT-02 | — | Copy populates /root/.claude from /tmp/.claude at start | manual | container smoke test | N/A | ⬜ pending |
| 1-01-03 | 01 | 1 | MOUNT-03 | — | Binary files pass through without sed corruption | unit | `cd cli && go test ./... -run TestDockerSetupBinarySkip` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `cli/claude_mount_test.go` — stubs for MOUNT-01, MOUNT-03 (mount args, binary detection)

*MOUNT-02 (container-side copy) is integration-only — no unit stub needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| /root/.claude populated on container start | MOUNT-02 | Requires live Docker container | Run `agentjail` in a test project, exec `ls /root/.claude` inside container |
| Path translation rewrites host home → /root | MOUNT-02 | Requires live container with real ~/.claude | `grep -r "/home/$USER" /root/.claude` returns no matches after startup |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
