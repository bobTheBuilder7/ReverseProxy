package main

import "net/http"

func IfThenElse[T any](condition bool, a T, b T) T {
	if condition {
		return a
	}
	return b
}

func redirectToHTTPS(w http.ResponseWriter, req *http.Request) {
	http.Redirect(w, req, "https://"+req.Host+req.URL.String(), http.StatusMovedPermanently)
}
