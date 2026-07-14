package service

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestHandleDatasetUploadedNormalizesPartnerCSV(t *testing.T) {
	datasetID, jobID := uuid.NewString(), uuid.NewString()
	store := newFakeStore()
	store.objects["datasets/source.csv"] = []byte(dirtyRefundCSV + "purchase_1001,buyer_200,refund_req_3001,agent_001,203.84,199.57,clothing,changed_mind,yes,approved,no,64,2026-06-01 09:06:00\n")
	publisher := &fakePublisher{}
	service := NewServiceWithStore(newFakeRepo(), store, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)))
	service.ConfigureNormalizationDir(t.TempDir())
	body, err := json.Marshal(datasetUploadedEvent{DatasetID: datasetID, JobID: jobID, FilePath: "datasets/source.csv"})
	if err != nil {
		t.Fatal(err)
	}

	if err := service.HandleRabbitEvent(context.Background(), DatasetUploadedRoutingKey, body); err != nil {
		t.Fatalf("handle dataset.uploaded: %v", err)
	}
	if len(publisher.messages) != 1 || publisher.messages[0].routingKey != DatasetNormalizedRoutingKey {
		t.Fatalf("published messages = %+v", publisher.messages)
	}
	event, ok := publisher.messages[0].payload.(normalizedDatasetEvent)
	if !ok {
		t.Fatalf("payload type = %T", publisher.messages[0].payload)
	}
	if event.DatasetID != datasetID || event.JobID != jobID || event.RecordCount != 2 || event.SchemaVersion != normalizedSchemaVersion {
		t.Fatalf("normalized event = %+v", event)
	}

	file, err := os.Open(event.RecordsPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || strings.Join(rows[0], ",") != strings.Join(normalizedColumns, ",") {
		t.Fatalf("normalized rows = %+v", rows)
	}
	if rows[1][0] != "order_1001" || rows[1][1] != "customer_200" || rows[1][2] != "return_3001" {
		t.Fatalf("identifiers were not normalized: %+v", rows[1][:3])
	}
	if rows[1][8] != "true" || rows[1][9] != "APPROVED" || rows[1][10] != "false" || rows[1][12] != "2026-06-01T09:06:00Z" {
		t.Fatalf("values were not normalized: %+v", rows[1])
	}
	if event.RecordsPath != filepath.Join(service.normalizedDir, datasetID+".csv") {
		t.Fatalf("recordsPath = %q", event.RecordsPath)
	}
}

func TestHandleDatasetUploadedPublishesNormalizationFailure(t *testing.T) {
	datasetID, jobID := uuid.NewString(), uuid.NewString()
	publisher := &fakePublisher{}
	service := NewServiceWithStore(newFakeRepo(), newFakeStore(), publisher, slog.New(slog.NewTextHandler(io.Discard, nil)))
	service.ConfigureNormalizationDir(t.TempDir())
	body, _ := json.Marshal(datasetUploadedEvent{DatasetID: datasetID, JobID: jobID, FilePath: "datasets/missing.csv"})

	if err := service.HandleDatasetUploaded(context.Background(), body); err != nil {
		t.Fatalf("handle missing source: %v", err)
	}
	if len(publisher.messages) != 1 || publisher.messages[0].routingKey != PipelineFailedRoutingKey {
		t.Fatalf("published messages = %+v", publisher.messages)
	}
	failure, ok := publisher.messages[0].payload.(normalizationFailedEvent)
	if !ok || failure.Stage != "NORMALIZATION" || failure.DatasetID != datasetID || failure.JobID != jobID {
		t.Fatalf("failure event = %#v", publisher.messages[0].payload)
	}
}
