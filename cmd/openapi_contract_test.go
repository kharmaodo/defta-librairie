package main

import (
	"defta-librairie/internal/models"
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func loadOpenAPI(t *testing.T) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile("../static/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]interface{}
	if err = json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}
func TestOpenAPIContractMatchesRoutes(t *testing.T) {
	doc := loadOpenAPI(t)
	if doc["openapi"] != "3.0.3" {
		t.Fatal("unexpected OpenAPI version")
	}
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`mux\.Handle(?:Func)?\("(GET|POST|PUT|PATCH|DELETE) (/api/[^" ]+)"`)
	expected := map[string]bool{}
	for _, m := range pattern.FindAllStringSubmatch(string(source), -1) {
		expected[strings.ToLower(m[1])+" "+m[2]] = true
	}
	paths := doc["paths"].(map[string]interface{})
	seen := map[string]bool{}
	ids := map[string]bool{}
	for path, raw := range paths {
		for method, value := range raw.(map[string]interface{}) {
			key := method + " " + path
			if !expected[key] {
				t.Errorf("documented route not registered: %s", key)
			}
			seen[key] = true
			op := value.(map[string]interface{})
			id, ok := op["operationId"].(string)
			if !ok || id == "" || ids[id] {
				t.Errorf("missing/duplicate operationId: %s", key)
			}
			ids[id] = true
			if _, ok := op["responses"].(map[string]interface{}); !ok {
				t.Errorf("missing responses: %s", key)
			}
			if samples, ok := op["x-codeSamples"].([]interface{}); !ok || len(samples) == 0 {
				t.Errorf("missing curl: %s", key)
			}
			for _, match := range regexp.MustCompile(`\{(\w+)\}`).FindAllStringSubmatch(path, -1) {
				found := false
				if parameters, ok := op["parameters"].([]interface{}); ok {
					for _, p := range parameters {
						v := p.(map[string]interface{})
						if v["name"] == match[1] && v["in"] == "path" && v["required"] == true {
							found = true
						}
					}
				}
				if !found {
					t.Errorf("path parameter missing: %s", key)
				}
			}
			if response, ok := op["responses"].(map[string]interface{})["204"]; ok {
				if _, exists := response.(map[string]interface{})["content"]; exists {
					t.Errorf("204 has body: %s", key)
				}
			}
		}
	}
	for key := range expected {
		if !seen[key] {
			t.Errorf("route missing from OpenAPI: %s", key)
		}
	}
	var walk func(interface{})
	walk = func(value interface{}) {
		switch v := value.(type) {
		case map[string]interface{}:
			if ref, ok := v["$ref"].(string); ok {
				if !strings.HasPrefix(ref, "#/") {
					t.Errorf("external reference: %s", ref)
				} else {
					var target interface{} = doc
					for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
						m, ok := target.(map[string]interface{})
						if !ok {
							target = nil
							break
						}
						target = m[part]
					}
					if target == nil {
						t.Errorf("unresolved reference: %s", ref)
					}
				}
			}
			for _, child := range v {
				walk(child)
			}
		case []interface{}:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(doc)
}
func TestOpenAPIModelFields(t *testing.T) {
	doc := loadOpenAPI(t)
	schemas := doc["components"].(map[string]interface{})["schemas"].(map[string]interface{})
	for _, value := range []interface{}{models.Book{}, models.BookInput{}, models.Sale{}, models.SaleInput{}, models.Purchase{}, models.PurchaseInput{}, models.Customer{}, models.Supplier{}, models.CashRegister{}, models.Payment{}, models.ReturnSettlement{}, models.CustomerReturn{}, models.SupplierReturn{}, models.LibrarySettings{}, models.BookInventory{}, models.AuditLog{}, models.ActiveSession{}} {
		typ := reflect.TypeOf(value)
		schema, ok := schemas[typ.Name()].(map[string]interface{})
		if !ok {
			t.Errorf("missing schema %s", typ.Name())
			continue
		}
		properties := schema["properties"].(map[string]interface{})
		expected := map[string]bool{}
		for i := 0; i < typ.NumField(); i++ {
			tag := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
			if tag != "" && tag != "-" {
				expected[tag] = true
				if _, ok := properties[tag]; !ok {
					t.Errorf("%s.%s missing", typ.Name(), tag)
				}
			}
		}
		for key := range properties {
			if !expected[key] {
				t.Errorf("unknown field %s.%s", typ.Name(), key)
			}
		}
	}
}
