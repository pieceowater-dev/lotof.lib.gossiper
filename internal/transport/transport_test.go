package transport

import "testing"

func TestFactory_CreateTransport(t *testing.T) {
	f := NewFactory()

	t.Run("grpc type returns a GRPCTransport", func(t *testing.T) {
		got := f.CreateTransport(GRPC, "localhost:9000")
		grpcTransport, ok := got.(*GRPCTransport)
		if !ok {
			t.Fatalf("expected *GRPCTransport, got %T", got)
		}
		if grpcTransport.address != "localhost:9000" {
			t.Errorf("expected address %q, got %q", "localhost:9000", grpcTransport.address)
		}
	})

	t.Run("unknown type returns nil", func(t *testing.T) {
		got := f.CreateTransport(Type("unknown"), "localhost:9000")
		if got != nil {
			t.Errorf("expected nil transport, got %v", got)
		}
	})
}
