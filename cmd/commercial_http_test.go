package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"defta-librairie/internal/auth"
	"defta-librairie/internal/handlers"
	"defta-librairie/internal/middleware"
	"defta-librairie/internal/migrations"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	_ "github.com/mattn/go-sqlite3"
)

func TestCommercialHTTPLifecycle(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "http.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrations.Run(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(commercialHTTPFixture); err != nil {
		t.Fatal(err)
	}
	tokens, err := auth.NewTokenManager(strings.Repeat("test-only-", 8), "acceptance", "acceptance", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	issue := func(id, library string, role models.UserRole, change bool) string {
		t.Helper()
		sid := id
		if change {
			sid += "-change"
		}
		expiry := time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano)
		if _, err := db.Exec(`INSERT INTO refresh_sessions(id,user_id,token_hash,token_family,expires_at,created_at) VALUES(?,?,?,?,?,?)`, sid, id, sid, sid, expiry, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
		token, _, err := tokens.IssueForSession(models.User{ID: id, Role: role, LibraryID: library, MustChangePassword: change}, sid)
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	owner := issue("http-owner", "http-library", models.RoleOwnerLibrary, false)
	other := issue("http-other", "http-other-library", models.RoleOwnerLibrary, false)
	root := issue("http-root", "", models.RoleSuperAdminRoot, false)
	change := issue("http-owner", "http-library", models.RoleOwnerLibrary, true)
	protected := func(h http.Handler) http.Handler {
		return middleware.AuthenticateSession(tokens, repositories.NewSessionRepository(db), middleware.RequirePasswordChanged(middleware.RequireRoles(h, models.RoleSuperAdminRoot, models.RoleOwnerLibrary)))
	}
	mux := http.NewServeMux()
	registerCommercialHTTPRoutes(mux, protected,
		handlers.NewCommercialStatisticsHandler(services.NewCommercialStatisticsService(repositories.NewCommercialStatisticsRepository(db))),
		handlers.NewSaleHandler(services.NewSaleService(repositories.NewSaleRepository(db))),
		handlers.NewPaymentHandler(services.NewPaymentService(repositories.NewPaymentRepository(db))),
		handlers.NewCustomerReturnHandler(services.NewCustomerReturnService(repositories.NewCustomerReturnRepository(db))),
		handlers.NewReturnSettlementHandler(services.NewReturnSettlementService(repositories.NewReturnSettlementRepository(db))))
	app := middleware.SecureHTTP(mux)
	request := func(method, path, token string, body interface{}, status int) map[string]interface{} {
		t.Helper()
		var buf bytes.Buffer
		if body != nil {
			if err := json.NewEncoder(&buf).Encode(body); err != nil {
				t.Fatal(err)
			}
		}
		req := httptest.NewRequest(method, path, &buf)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		response := httptest.NewRecorder()
		app.ServeHTTP(response, req)
		if response.Code != status {
			t.Fatalf("%s %s: status=%d want=%d body=%s", method, path, response.Code, status, response.Body.String())
		}
		if !strings.Contains(response.Header().Get("Content-Type"), "application/json") {
			t.Fatalf("non-JSON response: %s %s", method, path)
		}
		var value map[string]interface{}
		if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	assertError := func(v map[string]interface{}, code string) {
		t.Helper()
		if v["error"] != code {
			t.Fatalf("error=%v want=%s", v, code)
		}
	}
	number := func(v map[string]interface{}, key string, want float64) {
		t.Helper()
		got, ok := v[key].(float64)
		if !ok || math.IsNaN(got) || math.Abs(got-want) > 1e-9 {
			t.Fatalf("%s=%v want=%v", key, v[key], want)
		}
	}
	id := func(v map[string]interface{}) string {
		t.Helper()
		s, ok := v["id"].(string)
		if !ok || s == "" {
			t.Fatalf("missing id: %v", v)
		}
		return s
	}
	assertError(request("GET", "/api/manage/sales", "", nil, 401), "unauthorized")
	assertError(request("GET", "/api/manage/sales", owner+"invalid", nil, 401), "unauthorized")
	assertError(request("GET", "/api/manage/sales", change, nil, 403), "password_change_required")
	sale := request("POST", "/api/manage/sales", owner, map[string]interface{}{"customerName": "Recette HTTP", "lines": []map[string]interface{}{{"bookId": 1, "quantity": 4}}}, 201)
	saleID := id(sale)
	number(sale, "totalAmount", 6000)
	assertError(request("GET", "/api/manage/sales/"+saleID, other, nil, 404), "sale_not_found")
	sale = request("POST", "/api/manage/sales/"+saleID+"/confirm", owner, map[string]interface{}{"version": sale["version"]}, 200)
	paymentBody := map[string]interface{}{"cashRegisterId": "http-register", "method": "CASH", "amount": 1000}
	request("POST", "/api/manage/sales/"+saleID+"/payments", other, paymentBody, 404)
	request("POST", "/api/manage/sales/"+saleID+"/payments", owner, paymentBody, 201)
	b := request("GET", "/api/manage/sales/"+saleID+"/payment-balance", owner, nil, 200)
	number(b, "paidAmount", 1000)
	number(b, "remainingAmount", 5000)
	lines, ok := sale["lines"].([]interface{})
	if !ok || len(lines) != 1 {
		t.Fatalf("sale lines: %v", sale)
	}
	line, ok := lines[0].(map[string]interface{})
	if !ok {
		t.Fatal("invalid line")
	}
	ret := request("POST", "/api/manage/customer-returns", owner, map[string]interface{}{"saleId": saleID, "reason": "Retour recette HTTP", "resolution": "REFUND", "lines": []map[string]interface{}{{"saleLineId": id(line), "quantity": 1}}}, 201)
	retID := id(ret)
	ret = request("POST", "/api/manage/customer-returns/"+retID+"/complete", owner, map[string]interface{}{"version": ret["version"]}, 200)
	base := "/api/manage/customer-returns/" + retID
	number(request("GET", base+"/settlement-balance", owner, nil, 200), "refundableAmount", 1000)
	request("GET", base+"/settlement-balance", other, nil, 404)
	assertError(request("POST", base+"/settlements", owner, map[string]interface{}{"method": "CASH", "amount": 1001}, 409), "refund_exceeds_payments")
	request("POST", base+"/settlements", owner, map[string]interface{}{"method": "CASH", "amount": 700}, 201)
	number(request("GET", base+"/settlement-balance", owner, nil, 200), "refundableAmount", 300)
	paymentBody["amount"] = 5000
	request("POST", "/api/manage/sales/"+saleID+"/payments", owner, paymentBody, 201)
	request("POST", base+"/settlements", owner, map[string]interface{}{"method": "CASH", "amount": 800}, 201)
	b = request("GET", base+"/settlement-balance", owner, nil, 200)
	number(b, "refundableAmount", 0)
	number(b, "settledAmount", 1500)
	number(b, "remainingAmount", 0)
	assertError(request("POST", base+"/settlements", owner, map[string]interface{}{"method": "CASH", "amount": 1}, 409), "return_settlement_exceeds_balance")
	request("POST", base+"/complete", owner, map[string]interface{}{"version": ret["version"]}, 409)
	request("POST", "/api/manage/sales/"+saleID+"/cancel", owner, map[string]interface{}{"version": sale["version"]}, 409)
	stats := "/api/manage/statistics?from=2000-01-01T00:00:00Z&to=2100-01-01T00:00:00Z"
	v := request("GET", stats, owner, nil, 200)
	number(v, "netSales", 4500)
	number(v, "knownCost", 3000)
	number(v, "netMargin", 1500)
	request("GET", stats+"&libraryId=http-other-library", owner, nil, 403)
	request("GET", stats, root, nil, 400)
	number(request("GET", stats+"&libraryId=http-library", root, nil, 200), "netSales", 4500)
	number(request("GET", stats, other, nil, 200), "netSales", 0)
	var quantity int
	var cost float64
	if err = db.QueryRow("SELECT quantity,average_unit_cost FROM book_inventory WHERE book_id=1").Scan(&quantity, &cost); err != nil {
		t.Fatal(err)
	}
	if quantity != 7 || math.IsNaN(cost) || math.Abs(cost-1000) > 1e-9 {
		t.Fatalf("stock=%d CMP=%v", quantity, cost)
	}
	if _, err = db.Exec("UPDATE refresh_sessions SET revoked_at=? WHERE id='http-owner'", time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	assertError(request("GET", base+"/settlement-balance", owner, nil, 401), "unauthorized")
}

const commercialHTTPFixture = `
INSERT INTO users(id,username,password_hash,role,created_at,updated_at) VALUES
 ('http-owner','http-owner','unused','OWNER_LIBRARY','now','now'),
 ('http-other','http-other','unused','OWNER_LIBRARY','now','now'),
 ('http-root','http-root','unused','SUPER_ADMIN_ROOT','now','now');
INSERT INTO libraries(id,name,owner_user_id,created_at,updated_at) VALUES
 ('http-library','HTTP test','http-owner','now','now'),
 ('http-other-library','Other HTTP test','http-other','now','now');
INSERT INTO defta(id,title,price,library_id) VALUES(1,'Livre HTTP',1500,'http-library');
INSERT INTO book_inventory(book_id,library_id,quantity,average_unit_cost,version,updated_at)
 VALUES(1,'http-library',10,1000,1,'now');
INSERT INTO cash_registers(id,library_id,name,normalized_name,created_by,created_at,updated_at)
 VALUES('http-register','http-library','Caisse HTTP','caisse http','http-owner','now','now');
`
