package natsrouter

type HandlerError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func (e *HandlerError) Error() string {
	return e.Message
}

// Error configuration
// `Tag` is the header key and `Format` is the error encoding
// Defaults are "error" and "json"
type ErrorConfig struct {
	Tag    string
	Format string
}
