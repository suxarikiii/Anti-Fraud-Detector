# Contributing

Keep changes focused, tested, and easy to review. The repository is split into four working areas:

| Branch / area | Scope |
| --- | --- |
| `backend` | Services, APIs, business logic, and persistence |
| `frontend` | Analyst UI and browser-side behavior |
| `devops` | Docker, CI/CD, deployment, and infrastructure |
| `ml` | Data generation, normalization, anomaly detection, and metrics |

Use the matching area branch unless the team has agreed on a task-specific branch. Cross-area work may be split into separate pull requests when that makes ownership or review clearer.

## Before opening a pull request

1. Rebase or merge the latest target branch according to the team's workflow.
2. Run the checks for every area you changed.
3. Update API, event, configuration, or workflow documentation with the code.
4. State known limitations and follow-up work explicitly.
5. Open the pull request into `main` and request at least one review.

Useful checks:

```bash
(cd backend/upload-service && go test ./...)
(cd backend/relations-service && go test ./...)
(cd backend/scoring-service && ./gradlew clean test build)
(cd frontend && npm test && npm run build)
docker compose config --quiet
```

## Commit messages

Use a short imperative summary:

```text
type: description
```

Common types are `feat`, `fix`, `docs`, `test`, `refactor`, and `chore`.

Examples:

```text
feat: add dataset scoring export
fix: preserve failed pipeline context
docs: clarify relations event flow
```

## Pull request content

Include:

- the problem and the chosen solution;
- important files or contracts changed;
- commands run and their results;
- screenshots or API evidence when behavior is user-visible;
- migrations, compatibility notes, known gaps, and rollback considerations.

Do not include secrets, generated build output, or unrelated formatting changes.
