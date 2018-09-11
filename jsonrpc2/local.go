package jsonrpc2

import (
	"context"
)

var _ Service = &Local{}

// Local is a Service implementation for a local Server. It's like Remote, but
// no Codec.
type Local struct {
	Client
	Server
}

func (loc *Local) Call(ctx context.Context, result interface{}, method string, params ...interface{}) error {
	req, err := loc.Client.Request(method, params...)
	if err != nil {
		return err
	}
	ctx = context.WithValue(ctx, ctxService, loc)
	resp := loc.Server.Handle(ctx, req)
	return resp.UnmarshalResult(result)
}
