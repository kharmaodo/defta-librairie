package handlers

import (
	"fmt"
	"net/http"
)

func CatalogueHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, `
		<!DOCTYPE html>
		<html lang="ar" dir="rtl">
		<head>
			<meta charset="UTF-8">
			<title>Defta Librairie</title>
			<style>
				body { font-family: 'Noto Sans Arabic', sans-serif; text-align: center; padding: 4rem; }
				h1   { color: #2c3e50; }
			</style>
		</head>
		<body>
			<h1>مرحباً بك في كتالوج ديفتا</h1>
			<p>المشروع يعمل ! (version 0.1.0-dev)</p>
			<p><a href="/api/books?q=test&limit=5">Tester l'API books</a></p>
		</body>
		</html>
	`)
}

func APIBooksHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"message": "API books pas encore implémentée", "status": "ok"}`)
}