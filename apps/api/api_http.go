package main

import (
	"net/http"
	"strconv"
)

const defaultPageSize = 30
const maximumPageSize = 100

// readPagination defines the common bounded collection contract.
func readPagination(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	page, perPage := 1, defaultPageSize
	var err error
	if value := r.URL.Query().Get("page"); value != "" {
		page, err = strconv.Atoi(value)
		if err != nil || page < 1 {
			writeJSON(w, 422, map[string]string{"error": "invalid_pagination"})
			return 0, 0, false
		}
	}
	if value := r.URL.Query().Get("per_page"); value != "" {
		perPage, err = strconv.Atoi(value)
		if err != nil || perPage < 1 || perPage > maximumPageSize {
			writeJSON(w, 422, map[string]string{"error": "invalid_pagination"})
			return 0, 0, false
		}
	}
	return page, perPage, true
}

func paginate[T any](items []T, page, perPage int) []T {
	if len(items) == 0 || page > (len(items)+perPage-1)/perPage {
		return []T{}
	}
	start := (page - 1) * perPage
	end := start + perPage
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}
