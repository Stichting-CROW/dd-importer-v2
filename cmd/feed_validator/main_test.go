package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateHandlerRequiresToken(t *testing.T) {
	handler := validateHandler("secret")
	body := `{"data_type":"vehicles","feed_type":"gbfs","feed_url":"/relative"}`

	for _, c := range []struct {
		token string
		want  int
	}{{"", http.StatusUnauthorized}, {"wrong", http.StatusUnauthorized}, {"secret", http.StatusOK}} {
		request := httptest.NewRequest(http.MethodPost, "/validate", strings.NewReader(body))
		request.Header.Set(tokenHeader, c.token)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != c.want {
			t.Errorf("token %q: status %d, want %d", c.token, recorder.Code, c.want)
		}
	}
}

func TestValidateHandlerRejectsInvalidBody(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/validate", strings.NewReader("not json"))
	request.Header.Set(tokenHeader, "secret")
	recorder := httptest.NewRecorder()
	validateHandler("secret").ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status %d", recorder.Code)
	}
}
