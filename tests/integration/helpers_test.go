package integration_test

import (
	"net/http"
	"net/http/httptest"
)

type recorder struct{ *httptest.ResponseRecorder }

func newRecorder() recorder { return recorder{httptest.NewRecorder()} }

func (r recorder) status() int { return r.Code }

func newRequest(method, target string) *http.Request { return httptest.NewRequest(method, target, nil) }
