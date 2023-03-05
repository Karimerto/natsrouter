# NATS Router

NATS Router is a simple package of utilities on top of [NATS messaging system](https://nats.io/). It simplified the creation of queue groups and subjects, adds basic middleware support and adds both context and error handling to NATS subscriptions.

## Usage

To use NATS Router, first create a NatsRouter object and pass in a NATS connection:

    import "github.com/nats-io/nats.go"
    import "github.com/example/natsrouter"
    
    nc, err := nats.Connect(nats.DefaultURL)
    if err != nil {
    	// handle error
    }
    router := natsrouter.NewRouter(nc)

Next, register handlers for the different subjects you expect to receive:

    router.Subscribe("foo", func(msg *natsrouter.NatsMsg) error {
    	// Handle "foo" subject here
    	return nil
    })
    router.Subscribe("bar", func(msg *natsrouter.NatsMsg) error {
    	// Handle "bar" subject here
    	return nil
    })

You can use the `NatsMsg` object passed to the handler functions to inspect and manipulate the incoming message, including sending a response:

    router.Subscribe("ping", func(msg *natsrouter.NatsMsg) {
    	// Extract data from incoming message
    	data := msg.Data
    
    	// Send response
    	if err := msg.Respond([]byte("pong")); err != nil {
    		// handle error
    	}
    })

You can also subscribe using a queue group for distributed request processing:

    router.QueueSubscribe("baz", func(msg *natsrouter.NatsMsg) error {
    	// Handle "baz" subject here
    	return nil
    })

### Middlewares

The primary reason for `natsrouter` is middleware support for incoming request. These are modeled after generic HTTP middlewares, or any other similar middleware. There are a few example middlewares in [middleware.go](https://raw.githubcontent.com/Karimerto/natsrouter/blob/master/middleware.go).

Middleware can be layered on top of `NatsRouter` before subscribing to subjects. It is possible to build a complex tree if necessary. For example authentication can be performed with the simple `AuthMiddleware` example:

    cb := func(token string) bool {
    	// Test if token is valid
    	if token == "admin" {
    		return true
    	}
    	return false
    }
    auth := natsrouter.NewAuthMiddleware(cb)
    router.Use(auth.Auth).Subscribe("foo", func(msg *natsrouter.NatsMsg) error {
    	// The auth middleware check "authorization" header and errors out if authentication fails.
    	// This handler is only reached when authentication succeeds
    	return nil
    })

This is of course just a simple example. A real one would use for example JWTs or something similar. It must however be noted that NATS headers are limited to 4096 bytes in total, so keeping the tokens small is recommended.

### Subjects and queue groups

Instead of manually building subjects, it is possible to use `Subject()` to build any kind of subject chain:

    router.Subject("orders").Subject("eu").Subscribe(func(msg *natsrouter.NatsMsg) error {
    	// Handle "orders.eu" subject here
    	return nil
    })

Note that there is no subject in the `Subscribe()` function.

There is also the wildcard `*` and the any `>` subjects. If the "any" subject is included, it must be the last in chain.

Another helpful utility are queue groups. Both subjects and queue groups are mix-and-match, but only a single queue group will be in effect:

    router.Use(auth.Auth).Queue("my-group").Subject("orders").Subject("us").Subscribe(func(msg *natsrouter.NatsMsg) error {
    	// Handle "orders.us" subject here, in one of multiple instances belonging to "my-group".
    	// Also protected by "authentication" middleware.
    	return nil
    })

## Contributing

Contributions are welcome! If you find a bug or have a feature request, please [open an issue](https://github.com/Karimerto/natsrouter/issues/new). If you would like to contribute code, please fork the repository and create a pull request.
