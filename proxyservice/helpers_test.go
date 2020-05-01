package proxyservice

import (
	"io"
	"net/http"
	"net/http/httptest"
)

var fakeJenkins = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/crumbIssuer/api/json" {
		w.Write([]byte(`{"crumb": "crmb", "crumbRequestField": "Jenkins-Crumb"}`))
		return
	}
	w.WriteHeader(201)
}))


func sendRequest(method, url string, body io.Reader, h http.Handler) *httptest.ResponseRecorder {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		panic(err)
	}
	recorder := new(httptest.ResponseRecorder)
	h.ServeHTTP(recorder, req)
	return recorder
}
