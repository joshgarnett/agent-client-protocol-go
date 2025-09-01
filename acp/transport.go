package acp

import (
	"context"
	"io"

	"golang.org/x/exp/jsonrpc2"
)

// queuedCall represents a call waiting to be processed.
type queuedCall struct {
	ctx        context.Context
	method     string
	params     interface{}
	resultChan chan callResult
}

// callResult holds the result of a queued call.
type callResult struct {
	result interface{}
	err    error
}

// binder is an internal struct that implements jsonrpc2.Binder to
// associate the user-provided handler with the connection.
type binder struct {
	handler Handler
	core    *ConnectionCore
}

// Bind is called by the jsonrpc2 library to bind the handler to the connection.
func (b *binder) Bind(_ context.Context, _ *jsonrpc2.Connection) (jsonrpc2.ConnectionOptions, error) {
	wrappedHandler := func(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
		// We pass a dummy AgentConnection here since the Handler interface expects it
		// This will be cleaned up when we refactor the Handler interface
		dummyAgent := &AgentConnection{core: b.core}
		return b.handler.Handle(ctx, dummyAgent, req)
	}

	return jsonrpc2.ConnectionOptions{
		Handler: jsonrpc2.HandlerFunc(wrappedHandler),
	}, nil
}

// stdioDialer is a custom dialer that uses an existing io.ReadWriteCloser (like stdin/stdout).
type stdioDialer struct {
	rwc io.ReadWriteCloser
}

func (d stdioDialer) Dial(_ context.Context) (io.ReadWriteCloser, error) {
	return d.rwc, nil
}
