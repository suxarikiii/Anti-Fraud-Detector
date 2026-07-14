# Frontend release E2E checklist

## Automated browser contract test

- Command: `cd frontend && npm run test:e2e`
- Browsers: Chromium and Firefox
- Viewport: `1366x768`
- Flow: upload → completed status → approvals → details → analytics/graph → saved decision → CSV export
- Invariant: every dataset-scoped request uses the uploaded ID; no `/datasets/demo/` request is allowed.

## External VM success flow

Run only after the release commit is deployed.

1. Record the release commit, date, browser, and viewport.
2. Open the public frontend and verify Upload, Relations, and Scoring health endpoints.
3. Upload `data/dirty_business_refund_dataset.csv`.
4. Record `datasetId` and `jobId`; verify row count, preview, mapping, and warnings.
5. Start analysis and wait for automatic `COMPLETED` without manual PATCH.
6. Verify approvals request and every investigation request use the recorded `datasetId`.
7. Open one critical case and verify evidence, customer history, agent analytics, and relation graph.
8. Save an analyst action, outcome, note, and analyst ID; refresh and verify the saved decision reloads.
9. Export the current risk/outcome filter and verify the downloaded CSV filename and contents.
10. Refresh on the details page and verify dataset/job/return context is restored.
11. Capture upload, approvals, details, analytics, graph, decision, and export screenshots from this deployed flow.

## Controlled failure and recovery flow

1. Use an invalid CSV and capture the backend validation message.
2. Use a controlled pipeline failure and verify failed stage/reason is shown.
3. Verify no approvals or fabricated investigation data appear after failure.
4. Retry from the UI and verify the new `jobId` reaches `COMPLETED`.
5. Open dataset history and verify the failed and retried runs remain auditable.

## Accessibility and responsive pass

- Complete the critical path using keyboard only.
- Confirm visible focus on navigation, upload, filters, pagination, review, and graph-related controls.
- Verify risk is communicated by text as well as color.
- Check `1366x768`, `1440x900`, and `760px` narrow layout.
- Check long dataset IDs, return IDs, reasons, and table values.
- Verify graph has a text edge list, legend, empty state, and truncated state.

## Presentation fallback

If the VM is unreachable, show the recorded deployed screenshots/video and the saved external smoke/E2E output for the same release commit. Do not switch an uploaded UUID to demo data. An explicit `demo` dataset may be shown separately and must remain visibly labelled as demo.

## Evidence record

The following must be filled after deployment, not from local mocks:

- Release commit/tag:
- Deployment timestamp:
- Public URL:
- Chromium result:
- Firefox result:
- Screenshot/GIF/video paths:
- Known limitations:
