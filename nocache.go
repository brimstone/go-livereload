package main

import "net/http"

func nocache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=0, no-cache, no-store")
		h.ServeHTTP(w, r)
	})

}
