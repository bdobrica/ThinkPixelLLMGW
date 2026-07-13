//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"llm_gateway/internal/models"
	"llm_gateway/internal/storage"
)

func TestAdminUsageHandlerListsBoundedUsageAndBilling(t *testing.T) {
	skipIfNoDatabase(t)
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	keyID := uuid.New()
	if _, err := db.Conn().ExecContext(ctx, "INSERT INTO api_keys(id,name,key_hash,rate_limit_per_minute,enabled) VALUES($1,$2,$3,60,true)", keyID, "phase6-usage-test", uuid.New().String()); err != nil {
		t.Fatal(err)
	}
	defer db.Conn().ExecContext(ctx, "DELETE FROM api_keys WHERE id=$1", keyID)
	record := &models.UsageRecord{APIKeyID: keyID, RequestID: uuid.New(), ModelName: "phase6-test", Endpoint: "/v1/chat/completions", InputTokens: 10, OutputTokens: 5, StatusCode: 200}
	if err := storage.NewUsageRepository(db).Create(ctx, record); err != nil {
		t.Fatal(err)
	}
	defer db.Conn().ExecContext(ctx, "DELETE FROM usage_records WHERE id=$1", record.ID)
	now := time.Now().UTC()
	if _, err := db.Conn().ExecContext(ctx, `INSERT INTO monthly_usage_summary(id,api_key_id,year,month,total_requests,total_input_tokens,total_output_tokens,total_cost_nano_usd) VALUES($1,$2,$3,$4,1,10,5,123000000) ON CONFLICT(api_key_id,year,month) DO UPDATE SET total_requests=EXCLUDED.total_requests,total_input_tokens=EXCLUDED.total_input_tokens,total_output_tokens=EXCLUDED.total_output_tokens,total_cost_nano_usd=EXCLUDED.total_cost_nano_usd`, uuid.New(), keyID, now.Year(), int(now.Month())); err != nil {
		t.Fatal(err)
	}
	handler := NewAdminUsageHandler(db)
	usageReq := httptest.NewRequest(http.MethodGet, "/admin/usage?model=phase6-test", nil)
	usageRec := httptest.NewRecorder()
	handler.List(usageRec, usageReq)
	if usageRec.Code != http.StatusOK {
		t.Fatalf("usage status %d: %s", usageRec.Code, usageRec.Body)
	}
	var usage map[string]interface{}
	if err := json.Unmarshal(usageRec.Body.Bytes(), &usage); err != nil || usage["total_count"].(float64) < 1 {
		t.Fatalf("unexpected usage response: %s", usageRec.Body)
	}
	billReq := httptest.NewRequest(http.MethodGet, "/admin/billing/monthly?api_key_id="+keyID.String(), nil)
	billRec := httptest.NewRecorder()
	handler.Monthly(billRec, billReq)
	if billRec.Code != http.StatusOK {
		t.Fatalf("billing status %d: %s", billRec.Code, billRec.Body)
	}
	dashReq := httptest.NewRequest(http.MethodGet, "/admin/dashboard?hours=24", nil)
	dashRec := httptest.NewRecorder()
	handler.Dashboard(dashRec, dashReq)
	if dashRec.Code != http.StatusOK {
		t.Fatalf("dashboard status %d: %s", dashRec.Code, dashRec.Body)
	}
}
