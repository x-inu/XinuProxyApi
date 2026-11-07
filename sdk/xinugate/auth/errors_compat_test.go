package auth_test

import (
	"net/http"
	"testing"

	xinugateauth "github.com/x-inu/XinuGate/v7/sdk/xinugate/auth"
)

func TestErrorLegacyUnkeyedLiteralCompatibility(t *testing.T) {
	err := xinugateauth.Error{"code", "message", false, http.StatusRequestTimeout}

	if err.Code != "code" || err.Message != "message" || err.Retryable || err.HTTPStatus != http.StatusRequestTimeout {
		t.Fatalf("unexpected error fields: %#v", err)
	}
}
