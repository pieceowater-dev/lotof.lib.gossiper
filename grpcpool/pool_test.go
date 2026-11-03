package grpcpool

import (
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestNewPooledClientRoundRobins(t *testing.T) {
	p, err := NewPooledClient("passthrough:///localhost:1", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if len(p.conns) != PoolSize {
		t.Fatalf("pool has %d connections", len(p.conns))
	}
	seen := map[*grpc.ClientConn]int{}
	for i := 0; i < PoolSize*3; i++ {
		seen[p.next()]++
	}
	if len(seen) != PoolSize {
		t.Fatalf("round robin touched %d of %d connections", len(seen), PoolSize)
	}
	for _, n := range seen {
		if n != 3 {
			t.Fatalf("uneven spread: %v", n)
		}
	}
}
