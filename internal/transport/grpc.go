package transport

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"reflect"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCTransport handles both client and server-side transport
type GRPCTransport struct {
	address string
	server  *grpc.Server
}

func NewGRPCTransport(address string) *GRPCTransport {
	return &GRPCTransport{address: address}
}

// CreateClient dynamically creates a gRPC client using the passed constructor with retry logic.
func (g *GRPCTransport) CreateClient(clientConstructor any) (any, error) {
	maxRetries := 8
	initialDelay := 3 * time.Second
	dialTimeout := 60 * time.Second

	var conn *grpc.ClientConn
	var err error

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Dial the gRPC connection with longer timeout
		ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
		conn, err = grpc.DialContext(ctx, g.address,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock())
		cancel()

		if err == nil {
			// Connection successful
			break
		}

		if attempt < maxRetries-1 {
			// Calculate exponential delay: 3s, 6s, 12s, 24s, 48s, 96s (multiply by 2 each time)
			delay := initialDelay * time.Duration(1<<attempt)
			log.Printf("Failed to connect to gRPC server %s (attempt %d/%d): %v. Retrying in %v...",
				g.address, attempt+1, maxRetries, err, delay)
			time.Sleep(delay)
		} else {
			log.Printf("Failed to connect to gRPC server %s after %d attempts: %v",
				g.address, maxRetries, err)
			return nil, fmt.Errorf("failed to connect to gRPC server after %d retries: %w", maxRetries, err)
		}
	}

	// Use reflection to call the constructor function dynamically
	constructorValue := reflect.ValueOf(clientConstructor)
	if constructorValue.Kind() != reflect.Func {
		return nil, errors.New("clientConstructor must be a function")
	}

	// Call the constructor to create the client (pass the connection as argument)
	clientValues := constructorValue.Call([]reflect.Value{reflect.ValueOf(conn)})

	// Ensure that the client creation was successful and return the client
	if len(clientValues) > 0 {
		return clientValues[0].Interface(), nil
	}
	return nil, errors.New("failed to create client")
}

// Send sends a dynamic gRPC request based on method name and request type
func (g *GRPCTransport) Send(ctx context.Context, client any, serviceMethod string, request any) (any, error) {
	// Use reflection to get the method from the client dynamically
	clientValue := reflect.ValueOf(client)
	method := clientValue.MethodByName(serviceMethod)
	if !method.IsValid() {
		return nil, errors.New("invalid service method: " + serviceMethod)
	}

	// Ensure the request is passed as a reflect.Value
	reqValue := reflect.ValueOf(request)
	if reqValue.IsValid() {
		// Call the method dynamically, passing the context and the request
		returnValues := method.Call([]reflect.Value{reflect.ValueOf(ctx), reqValue})
		if len(returnValues) > 1 && returnValues[1].Interface() != nil {
			return nil, returnValues[1].Interface().(error)
		}
		// Return the response from the method call
		return returnValues[0].Interface(), nil
	}
	return nil, errors.New("invalid request type for method")
}
