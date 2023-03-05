Here are a few example middlewares for `natsrouter`. The middlewares are:

 * `auth.go`: Authentication middleware that authenticates the message with an authorization token using the header tag 'authorization' or a specified header tag.
 * `requestid.go`: Request ID middleware that parses the header for a request id (storing it into the request context), or generates a unique ID for each request using the header tag 'request_id' or a specified header tag.
 * `metrics.go`: Metrics middleware that collects Prometheus metrics for NATS messages including the total number of messages received, duration of requests, and size of payloads.
