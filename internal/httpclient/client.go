package httpclient

import (
	"net/http"
	"time"
)

const DefaultTimeout = 10 * time.Minute

func Default() *http.Client {
	return &http.Client{Timeout: DefaultTimeout}
}
