package jsonrpc2

import (
	"encoding/json"
	"net"
	"reflect"
	"testing"
)

type Foo struct{}

func (f *Foo) Ping() string {
	return "ping"
}

type Bar struct{}

func (b *Bar) Pong() string {
	return "pong"
}

func TestBidirectionalService(t *testing.T) {
	f := &Foo{}
	b := &Bar{}

	fooServer := Server{}
	if err := fooServer.Register("foo_", f); err != nil {
		t.Fatal(err)
	}

	barServer := Server{}
	if err := barServer.Register("bar_", b); err != nil {
		t.Fatal(err)
	}

	fooPipe, barPipe := net.Pipe()
	defer fooPipe.Close()
	defer barPipe.Close()

	fooClient := Client{}
	//barClient := Client{}

	reqClient, err := fooClient.Request("bar_pong")
	if err != nil {
		t.Fatal(err)
	}
	go json.NewEncoder(fooPipe).Encode(reqClient)

	var reqServer Request
	if err := json.NewDecoder(barPipe).Decode(&reqServer); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(reqClient, &reqServer) {
		t.Errorf("server/client request mismatch: %q != %q", reqServer, reqClient)
	}

	resp := barServer.Handle(&reqServer)
	if string(resp.ID) != string(reqClient.ID) {
		t.Errorf("server/client request ID mismatch: %s", resp)
	}

	go json.NewEncoder(barPipe).Encode(resp)

	var respClient Response
	if err := json.NewDecoder(fooPipe).Decode(&respClient); err != nil {
		t.Fatal(err)
	}

	if respClient.Error != nil {
		t.Errorf("unexpected response error: %q", respClient)
	}

	var got string
	if err := json.Unmarshal(respClient.Result, &got); err != nil {
		t.Fatal(err)
	}

	if want := "pong"; got != want {
		t.Errorf("got %q; want %q;", got, want)
	}
}
