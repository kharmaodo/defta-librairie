package repositories

import (
	"database/sql"
	"math"
	"testing"
)

func TestReceiptAverageCost(t *testing.T) {
	for _, tc := range []struct{name string; before int; cost sql.NullFloat64; received int; price float64; want sql.NullFloat64}{
		{"weighted",10,sql.NullFloat64{Float64:1500,Valid:true},5,1800,sql.NullFloat64{Float64:1600,Valid:true}},
		{"unknown_opening",10,sql.NullFloat64{},5,1800,sql.NullFloat64{}},
		{"empty_stock",0,sql.NullFloat64{},5,1800,sql.NullFloat64{Float64:1800,Valid:true}},
		{"free_purchase",0,sql.NullFloat64{},5,0,sql.NullFloat64{Float64:0,Valid:true}},
		{"depleted_stock",0,sql.NullFloat64{Float64:1500,Valid:true},5,1800,sql.NullFloat64{Float64:1800,Valid:true}},
	} {
		t.Run(tc.name,func(t *testing.T){got:=receiptAverageCost(tc.before,tc.cost,tc.received,tc.price);if got.Valid!=tc.want.Valid || (got.Valid && math.Abs(got.Float64-tc.want.Float64)>1e-8){t.Fatalf("got %+v want %+v",got,tc.want)}})
	}
}
