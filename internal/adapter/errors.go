package adapter

import "fmt"

type ErrorClass int

const (
	Success ErrorClass = iota
	ClientError
	AuthError
	ConfigError
	RateLimited
	ServerError
	Timeout
	Network
	ClientCanceled
	BotDetected
	BudgetExceeded
)

type Error struct {
	Class      ErrorClass
	UpstreamID string
	Provider   string
	HTTPStatus int
	Message    string
	Retryable  bool
	Wrapped    error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.HTTPStatus != 0 {
		return fmt.Sprintf("%s upstream %s returned HTTP %d: %s", e.Provider, e.UpstreamID, e.HTTPStatus, e.Message)
	}
	return fmt.Sprintf("%s upstream %s: %s", e.Provider, e.UpstreamID, e.Message)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Wrapped
}

func ClassifyHTTP(status int) ErrorClass {
	switch {
	case status == 401 || status == 403:
		return AuthError
	case status == 404:
		return ConfigError
	case status == 408 || status == 429:
		return RateLimited
	case status >= 400 && status < 500:
		return ClientError
	case status >= 500:
		return ServerError
	default:
		return Success
	}
}

func Retryable(class ErrorClass) bool {
	return class == RateLimited || class == ServerError || class == Timeout || class == Network
}
