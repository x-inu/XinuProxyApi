package auth

import (
	"net/http"
	"testing"

	xinugateexecutor "github.com/x-inu/XinuGate/v7/sdk/xinugate/executor"
)

func TestRequestTerminatedErrorSkipsCreditsFallback(t *testing.T) {
	errTerminated := &xinugateexecutor.RequestTerminatedError{HTTPStatus: http.StatusTooManyRequests}
	if !isRequestTerminatedError(errTerminated) {
		t.Fatal("isRequestTerminatedError() = false")
	}
	if shouldAttemptAntigravityCreditsFallback(&Manager{}, errTerminated, []string{"antigravity"}) {
		t.Fatal("terminated request must not use Antigravity credits fallback")
	}
}
