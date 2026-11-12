# ADR 0013: Git flow with develop and squash merges

* Status: Accepted
* Date: 2026-11-13
* Supersedes: [ADR 0006](0006-trunk-based-development.md)

## Context and Problem Statement

ADR 0006 required every commit to be atomic, to build and to pass tests, with rebase merges onto `main`. In
practice, splitting finished work into atomic commits after the fact took more time and effort than the work itself,
and `main` mixed released and unreleased changes. FUI needs a separate preview stage before production and a lighter
day-to-day flow.

## Decision Outcome

- `main` is PROD (released versions); `develop` is PREVIEW (integrated development). Both are protected: changes
  land only through pull requests whose required CI checks are green.
- Feature branches start from `develop`; hotfix branches start from `main`. Both are squash-merged, so the PR title
  (a Conventional Commit) becomes the single commit on the target branch. After a hotfix, `main` is merged back into
  `develop`.
- Commits on feature and hotfix branches no longer have to be atomic. A versioned `pre-commit` hook (`make hooks`)
  runs format, build, vet, lint and tests so that no commit is broken.
- A release is a pull request from `develop` to `main`, merged with a merge commit, followed by the version bump:
  an annotated `vX.Y.Z` tag on `main`.

### Consequences

* **Positive:** less work before each merge; one commit per topic on the stable branches; a preview stage before
  production.
* **Negative:** history within a topic is lost on squash, so `git bisect` points to a whole topic rather than a
  single step; `develop` must be kept in sync after every hotfix.
