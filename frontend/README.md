# Frontend

React and TypeScript interface for the analyst workflow: upload a refund dataset, monitor processing, investigate suspicious approvals, record a decision, and export results.

## Responsibilities

- validate the selected CSV and show an upload preview;
- start analysis and poll the pipeline status;
- filter suspicious approvals by risk, agent, and investigation outcome;
- show scoring reasons and relationship context;
- save investigation decisions and download a filtered CSV export.

## Service interactions

```mermaid
flowchart LR
    Analyst[Analyst] --> UI[React Frontend]
    UI -->|/api/*| Nginx[Frontend Nginx]
    Nginx --> Gateway[API Gateway]
    Gateway --> Upload[Upload Service]
    Gateway --> Relations[Relations Service]
    Gateway --> Scoring[Scoring Service]
```

The browser uses relative `/api` URLs. In Docker, the frontend Nginx container forwards them to `gateway:8080`, so backend service addresses never leak into browser configuration.

## Workflow

```mermaid
sequenceDiagram
    actor A as Analyst
    participant UI as Frontend
    participant G as Gateway
    participant U as Upload Service
    participant S as Scoring Service

    A->>UI: Select CSV
    UI->>G: POST /api/datasets/upload
    G->>U: Upload and validate
    U-->>UI: datasetId and jobId
    UI->>G: GET preview
    A->>UI: Start analysis
    UI->>G: POST analysis start
    loop Until completed or failed
        UI->>G: GET job status
        G-->>UI: stage and progress
    end
    UI->>G: GET suspicious approvals
    A->>UI: Review and decide
    UI->>G: PUT investigation decision
    G->>S: Forward scoring requests
```

## Run locally

Requirements: Node.js 20+ and npm.

```bash
npm install
npm run dev
```

Vite serves the UI at `http://localhost:5173`. For a fully connected environment, start the root Docker Compose stack instead and open `http://localhost`.

## Verify

```bash
npm test
npm run build
```

## Key configuration

| File | Purpose |
| --- | --- |
| `src/App.tsx` | Application state and API workflow |
| `src/styles.css` | Product styles and responsive layout |
| `nginx.conf` | Static hosting and `/api` proxy |
| `vite.config.ts` | Development and test configuration |

The frontend does not store credentials or business data. Persistent state belongs to the backend services.
