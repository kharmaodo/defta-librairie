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
		</head>
		<body style="font-family: 'Noto Sans Arabic', sans-serif; text-align: center; padding: 4rem;">
			<h1>مرحباً بك في كتالوج ديفتا</h1>
			<p>الصفحة الرئيسية تعمل (v0.1)</p>
			<p><a href="/api/books?limit=5">→ Tester API /api/books</a></p>
		</body>
		</html>
	`)
}