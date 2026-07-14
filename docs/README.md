# Documentation index

Use this page to find the source of truth without duplicating details across files.

| Document | Use it for |
| --- | --- |
| [Root README](../README.md) | Project overview, quick start, and service map |
| [Architecture](architecture.md) | Boundaries, ownership, dependencies, and end-to-end flow |
| [API contracts](api-contracts.md) | HTTP payloads, errors, RabbitMQ events, and versioning |
| [Data format](data-format.md) | Input CSV schema, validation, and normalized records |
| [Scoring rules](scoring-rules.md) | Risk factors, thresholds, and explainability |
| [DevOps](devops.md) | Build, deployment, rollback, health checks, and operations |
| [Demo flow](demo-flow.md) | Repeatable product demonstration |
| [Release readiness](release-readiness.md) | Evidence, known gaps, and go-live checklist |
| [Week 7 final-report outline](week7-final-report-outline.md) | Follow-up reporting plan and evidence checklist |
| [License](../LICENSE.md) | Reuse terms for source code and documentation |

## Service documentation

- [Frontend](../frontend/README.md)
- [API Gateway](../backend/gateway/README.md)
- [Upload Service](../backend/upload-service/README.md)
- [Relations Service](../backend/relations-service/README.md)
- [Scoring Service](../backend/scoring-service/README.md)

Each service README follows the same compact structure: responsibilities, Mermaid interaction diagram, Mermaid workflow, API or routes, configuration, and verification.

## Documentation rules

- Keep one source of truth for each fact and link to it elsewhere.
- Describe current behavior; track future work explicitly as a gap or plan.
- Update the relevant service README when its API, event, dependency, or run command changes.
- Prefer short tables and diagrams over repeated prose.
