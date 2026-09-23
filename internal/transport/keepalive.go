package transport

import (
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

const (
	// ClientKeepaliveTime is how often an idle connection pings the server.
	// It must stay at or above the server's keepalive EnforcementPolicy
	// MinTime (see gossiper.NewGRPCServer), or the server answers GOAWAY
	// "too_many_pings" and kills the connection.
	ClientKeepaliveTime = 30 * time.Second
	// ClientKeepaliveTimeout is how long a ping waits for its ack before the
	// connection is declared dead and re-established.
	ClientKeepaliveTimeout = 10 * time.Second
)

// ClientKeepalive pings the server on idle connections too. Without it a
// connection to a pod that is already gone stays "ready" in the client's eyes
// -- nothing tells it otherwise until a call is made and runs into its own
// deadline. That is what turned every msvc restart into a batch of
// DeadlineExceeded errors in the calling gateway: the pool kept handing out a
// dead connection. With keepalive the client notices within
// ClientKeepaliveTime + ClientKeepaliveTimeout and reconnects on its own.
func ClientKeepalive() grpc.DialOption {
	return grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:                ClientKeepaliveTime,
		Timeout:             ClientKeepaliveTimeout,
		PermitWithoutStream: true,
	})
}
