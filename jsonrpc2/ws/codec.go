package ws

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/vipnode/vipnode/jsonrpc2"
)

type rwc struct {
	io.Reader
	io.Writer
	io.Closer
}

// WebSocketDial returns a Codec that wraps a client-side connection with JSON
// encoding and decoding.
func WebSocketDial(ctx context.Context, url string) (jsonrpc2.Codec, error) {
	conn, _, _, err := ws.Dial(ctx, url)
	if err != nil {
		return nil, err
	}

	return clientWebSocketCodec(conn), nil
}

func clientWebSocketCodec(conn net.Conn) jsonrpc2.Codec {
	r := wsutil.NewReader(conn, ws.StateClientSide)
	w := wsutil.NewWriter(conn, ws.StateClientSide, ws.OpText)
	return &wsCodec{
		inner: jsonrpc2.IOCodec(rwc{r, w, conn}),
		r:     r,
		w:     w,
	}
}

// WebSocketCodec returns a server-side Codec that wraps JSON encoding and
// decoding over a websocket connection.
func WebSocketCodec(conn net.Conn) jsonrpc2.Codec {
	r := wsutil.NewReader(conn, ws.StateServerSide)
	w := wsutil.NewWriter(conn, ws.StateServerSide, ws.OpText)
	return &wsCodec{
		inner: jsonrpc2.IOCodec(rwc{r, w, conn}),
		r:     r,
		w:     w,
	}
}

var _ jsonrpc2.Codec = &wsCodec{}

type wsCodec struct {
	inner jsonrpc2.Codec
	r     *wsutil.Reader
	w     *wsutil.Writer
}

func (codec *wsCodec) ReadMessage() (*jsonrpc2.Message, error) {
	_, err := codec.r.NextFrame()
	if err != nil {
		return nil, err
	}
	return codec.inner.ReadMessage()
}

func (codec *wsCodec) WriteMessage(msg *jsonrpc2.Message) error {
	err := codec.inner.WriteMessage(msg)
	if err != nil {
		return err
	}
	if err = codec.w.Flush(); err != nil {
		return err
	}
	return nil
}

func (codec *wsCodec) Close() error {
	return codec.inner.Close()
}

func WebsocketHandler(srv *jsonrpc2.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, _, _, err := ws.UpgradeHTTP(r, w, nil)
		if err != nil {
			log.Printf("websocket upgrade error from %s: %s", r.RemoteAddr, err)
			return
		}
		defer conn.Close()
		remote := &jsonrpc2.Remote{
			Codec:  WebSocketCodec(conn),
			Server: srv,
			Client: &jsonrpc2.Client{},

			// TODO: Unhardcode these?
			PendingLimit:   50,
			PendingDiscard: 10,
		}
		// FIXME: Connection is hijacked at this point, can't write the error.
		// Do we want to handle it somehow?
		if err := remote.Serve(); err != nil {
			log.Printf("jsonrpc2.Remote.Serve() error: %s", err)
		}
	}
}
