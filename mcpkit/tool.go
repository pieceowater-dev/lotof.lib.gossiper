package mcpkit

import (
	"context"
	"errors"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/grpc/status"
)

func ptr[T any](v T) *T { return &v }

// Annotations returns the hints a client shows for a class: Read is read-only, Write changes drafts only,
// Act can change what the public sees, so clients should ask the person first.
func Annotations(c Class) *sdk.ToolAnnotations {
	switch c {
	case Read:
		return &sdk.ToolAnnotations{ReadOnlyHint: true}
	case Act:
		return &sdk.ToolAnnotations{DestructiveHint: ptr(true), OpenWorldHint: ptr(true)}
	}
	return &sdk.ToolAnnotations{DestructiveHint: ptr(false)}
}

// AddTool registers a typed tool that is refused, before the handler runs, unless the key carries the scope of
// the class. Annotations default from the class. A gRPC error returned by the handler is turned into its plain
// message (see GRPCError).
func AddTool[In, Out any](srv *sdk.Server, p *Principal, class Class, t *sdk.Tool, h func(ctx context.Context, p *Principal, in In) (Out, error)) {
	if t.Annotations == nil {
		t.Annotations = Annotations(class)
	}
	sdk.AddTool(srv, t, func(ctx context.Context, _ *sdk.CallToolRequest, in In) (*sdk.CallToolResult, Out, error) {
		var zero Out
		if err := p.Need(string(class)); err != nil {
			return nil, zero, err
		}
		out, err := h(ctx, p, in)
		if err != nil {
			return nil, zero, GRPCError(err)
		}
		return nil, out, nil
	})
}

// GRPCError turns a gRPC status into a plain message the model can act on (no "rpc error: code = ..." noise).
func GRPCError(err error) error {
	if err == nil {
		return nil
	}
	if st, ok := status.FromError(err); ok {
		return errors.New(st.Message())
	}
	return err
}
