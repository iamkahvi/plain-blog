package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDirectoryRedirect(t *testing.T) {
	for _, tc := range []struct {
		request string
		want    string
	}{
		{"/pronounce", "/pronounce/"},
		{"/pronounce?font=geist&x=a%2Fb", "/pronounce/?font=geist&x=a%2Fb"},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, tc.request, nil)
		redirectDirectory(recorder, request, "/pronounce")
		if recorder.Code != http.StatusMovedPermanently || recorder.Header().Get("Location") != tc.want {
			t.Errorf("redirect %s = %d %q, want %d %q", tc.request, recorder.Code, recorder.Header().Get("Location"), http.StatusMovedPermanently, tc.want)
		}
	}
}
