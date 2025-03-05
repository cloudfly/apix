# APIX

Apix is a concise API development framework, which aim to develep a http api service fastly. 

It can easily integrated with grpc-gateway, integrating the interface of grpc into apix.

# Examples

```go

// simplest hello world sample.
apix.GET("/hello", func(ctx *apix.Context) (any, error) { return "Hello World", nil } )

// custom the response status code
apix.GET("/hello", func(ctx *apix.Context) (any, int, error) { return "Hello World", 200, nil } )

// bind with http.HandlerFunc
apix.GET("/hello", func(w ResponseWriter, r *Request) { w.Write("Hello World") } )

genService.RegisterYourServiceHandlerServer(context.Background(), apix.GRPCGatewayMux(), &ServiceImplements{})

```
