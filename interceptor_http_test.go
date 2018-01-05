package prisma

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/slover2000/prisma/logging"
	"github.com/slover2000/prisma/trace"
)

type fakeHTTPHandler struct {
}

func (h *fakeHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func TestHttpFilter(t *testing.T) {
	policy, _ := trace.NewLimitedSampler(100, 100)
	interceptorClient, _ := ConfigInterceptorClient(
		context.Background(),
		EnableTracing("test", policy, trace.NewConsoleCollector()),
		EnableLogging(logging.InfoLevel),
		EnableAllMetrics(),
		EnableMetricsExportHTTPServer(9100))

	fakeHandler := &fakeHTTPHandler{}
	handler := interceptorClient.HTTPHandler(fakeHandler, UsingFilter("/*", func(w http.ResponseWriter, r *http.Request) bool { trace.EnsureHttpSpan(r); return true }))

	req := httptest.NewRequest("GET", "http://example.com", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println(resp.StatusCode)
	fmt.Println(resp.Header.Get("Content-Type"))
	fmt.Println(string(body))
}
