package repositories

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestCommercialStatisticsEventsAndIsolation(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(statisticsFixture); err != nil {
		t.Fatal(err)
	}
	repo := NewCommercialStatisticsRepository(db)
	day := func(n int) time.Time { return time.Date(2026, 9, n, 0, 0, 0, 0, time.UTC) }
	for _, tc := range []struct {
		name, library                    string
		from, to                         int
		gross, cancelled, returned, cost float64
		unknown                          int
	}{
		{"sale day", "a", 1, 2, 200, 0, 0, 120, 0},
		{"return day", "a", 2, 3, 0, 0, 100, -60, 0},
		{"cancel day", "a", 4, 5, 0, 50, 0, -20, 0},
		{"unknown cost", "a", 5, 6, 30, 0, 0, 0, 1},
		{"known free", "a", 6, 7, 10, 0, 0, 0, 0},
		{"other library", "b", 1, 2, 900, 0, 0, 500, 0},
		{"empty", "missing", 1, 7, 0, 0, 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, err := repo.Summary(context.Background(), tc.library, day(tc.from), day(tc.to))
			if err != nil {
				t.Fatal(err)
			}
			if v.GrossSales != tc.gross || v.Cancellations != tc.cancelled || v.CustomerReturns != tc.returned || v.KnownCost != tc.cost || v.UnknownCostEvents != tc.unknown {
				t.Fatalf("unexpected summary: %+v", v)
			}
			if tc.unknown > 0 {
				if v.NetMargin != nil {
					t.Fatal("unknown cost must not produce a margin")
				}
			} else if v.NetMargin == nil || *v.NetMargin != tc.gross-tc.cancelled-tc.returned-tc.cost {
				t.Fatalf("incorrect margin: %+v", v)
			}
		})
	}
	if _, err = repo.Summary(context.Background(), "", day(1), day(2)); err == nil {
		t.Fatal("empty library accepted")
	}
	if _, err = repo.Summary(context.Background(), "a", day(2), day(1)); err == nil {
		t.Fatal("reversed interval accepted")
	}
}

const statisticsFixture = `
CREATE TABLE purchases(library_id TEXT,status TEXT,received_at TEXT,total_amount REAL);
CREATE TABLE supplier_returns(id INTEGER PRIMARY KEY,library_id TEXT,status TEXT,shipped_at TEXT,total_amount REAL);
CREATE TABLE supplier_return_lines(return_id INTEGER,quantity INTEGER,unit_cost_snapshot REAL);
CREATE TABLE sales(id TEXT,library_id TEXT,status TEXT,confirmed_at TEXT,cancelled_at TEXT);
CREATE TABLE sale_lines(id TEXT,sale_id TEXT,book_id INTEGER,quantity INTEGER,line_total REAL,unit_cost_snapshot REAL);
CREATE TABLE customer_returns(id TEXT,sale_id TEXT,library_id TEXT,status TEXT,completed_at TEXT);
CREATE TABLE customer_return_lines(return_id TEXT,sale_line_id TEXT,book_id INTEGER,quantity INTEGER,line_total REAL);
INSERT INTO sales VALUES
 ('s1','a','CONFIRMED','2026-09-01T00:00:00.123Z',NULL),
 ('s2','a','CANCELLED','2026-09-03T00:00:00Z','2026-09-04T00:00:00Z'),
 ('s3','a','CONFIRMED','2026-09-05T00:00:00Z',NULL),
 ('s4','a','CONFIRMED','2026-09-06T00:00:00Z',NULL),
 ('s5','b','CONFIRMED','2026-09-01T00:00:00Z',NULL),
 ('draft','a','DRAFT',NULL,NULL);
INSERT INTO sale_lines VALUES
 ('l1','s1',1,2,200,60),('l2','s2',2,1,50,20),('l3','s3',3,1,30,NULL),
 ('l4','s4',4,1,10,0),('l5','s5',5,1,900,500),('ld','draft',6,1,1000,NULL);
INSERT INTO customer_returns VALUES ('r','s1','a','COMPLETED','2026-09-02T00:00:00Z');
INSERT INTO customer_return_lines VALUES ('r','l1',1,1,100);
`

func TestCommercialStatisticsProcurement(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(statisticsFixture + procurementFixture); err != nil {
		t.Fatal(err)
	}
	repo := NewCommercialStatisticsRepository(db)
	for _, tc := range []struct {
		name, library      string
		day                int
		received, returned float64
	}{
		{"receipt and shipping", "a", 1, 300, 40},
		{"exclusive end and negative net", "a", 2, 0, 90},
		{"other library", "b", 1, 900, 200},
		{"empty", "missing", 1, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			from := time.Date(2026, 9, tc.day, 0, 0, 0, 0, time.UTC)
			v, err := repo.Summary(context.Background(), tc.library, from, from.AddDate(0, 0, 1))
			if err != nil {
				t.Fatal(err)
			}
			if v.ReceivedPurchases != tc.received || v.SupplierReturns != tc.returned || v.NetPurchases != tc.received-tc.returned {
				t.Fatalf("unexpected procurement: %+v", v)
			}
			if v.NetMargin == nil || *v.NetMargin != v.NetSales-v.KnownCost {
				t.Fatalf("procurement changed margin: %+v", v)
			}
		})
	}
}

const procurementFixture = `
INSERT INTO purchases VALUES
 ('a','RECEIVED','2026-09-01T00:00:00Z',100),
 ('a','RECEIVED','2026-09-02T01:00:00+02:00',200),
 ('a','DRAFT','2026-09-01T00:00:00Z',999),
 ('a','CANCELLED','2026-09-01T00:00:00Z',999),
 ('a','RECEIVED',NULL,999),
 ('b','RECEIVED','2026-09-01T00:00:00Z',900);
INSERT INTO supplier_returns(library_id,status,shipped_at,total_amount) VALUES
 ('a','SHIPPED','2026-09-01T00:00:00Z',40),
 ('a','SHIPPED','2026-09-02T00:00:00Z',90),
 ('a','DRAFT','2026-09-01T00:00:00Z',999),
 ('a','CANCELLED','2026-09-01T00:00:00Z',999),
 ('a','SHIPPED',NULL,999),
 ('b','SHIPPED','2026-09-01T00:00:00Z',200);
INSERT INTO supplier_return_lines SELECT id,1,NULL FROM supplier_returns;
`

func TestCommercialStatisticsSupplierReturnVariance(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(statisticsFixture + supplierVarianceFixture); err != nil {
		t.Fatal(err)
	}
	repo := NewCommercialStatisticsRepository(db)
	for _, tc := range []struct {
		name, library string
		day           int
		amount, known float64
		unknown       int
	}{
		{"multiple lines and timezone", "a", 1, 2500, 1800, 0},
		{"mixed known and unknown", "a", 2, 2000, 500, 1},
		{"known zero", "a", 3, 100, 0, 0},
		{"negative variance", "a", 4, 100, 150, 0},
		{"unknown sales independent", "a", 5, 0, 0, 0},
		{"empty period", "a", 7, 0, 0, 0},
		{"other library", "b", 1, 900, 600, 0},
		{"missing library", "missing", 1, 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			from := time.Date(2026, 9, tc.day, 0, 0, 0, 0, time.UTC)
			got, err := repo.Summary(context.Background(), tc.library, from, from.AddDate(0, 0, 1))
			if err != nil {
				t.Fatal(err)
			}
			if got.SupplierReturns != tc.amount || got.SupplierReturnKnownCost != tc.known || got.SupplierReturnUnknownCostLines != tc.unknown {
				t.Fatalf("unexpected supplier totals: %+v", got)
			}
			if tc.unknown > 0 {
				if got.SupplierReturnInventoryCost != nil || got.SupplierReturnCostVariance != nil {
					t.Fatal("partial costs presented as complete")
				}
			} else if got.SupplierReturnInventoryCost == nil || *got.SupplierReturnInventoryCost != tc.known || got.SupplierReturnCostVariance == nil || *got.SupplierReturnCostVariance != tc.amount-tc.known {
				t.Fatalf("incorrect supplier variance: %+v", got)
			}
			if got.UnknownCostEvents > 0 {
				if got.NetMargin != nil {
					t.Fatal("unknown sale cost must retain unknown margin")
				}
			} else if got.NetMargin == nil || *got.NetMargin != got.NetSales-got.KnownCost {
				t.Fatal("supplier variance altered sales margin")
			}
		})
	}
}

const supplierVarianceFixture = `
INSERT INTO supplier_returns VALUES
 (1,'a','SHIPPED','2026-09-02T01:00:00+02:00',2500),
 (2,'a','SHIPPED','2026-09-02T00:00:00Z',2000),
 (3,'a','SHIPPED','2026-09-03T00:00:00Z',100),
 (4,'a','SHIPPED','2026-09-04T00:00:00Z',100),
 (5,'b','SHIPPED','2026-09-01T00:00:00Z',900),
 (6,'a','DRAFT','2026-09-01T00:00:00Z',999),
 (7,'a','CANCELLED','2026-09-01T00:00:00Z',999),
 (8,'a','SHIPPED',NULL,999);
INSERT INTO supplier_return_lines VALUES
 (1,2,800),(1,1,200),
 (2,1,500),(2,1,NULL),
 (3,1,0),(4,1,150),(5,2,300),
 (6,1,NULL),(7,1,NULL),(8,1,NULL);
`
