package gojs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"sort"

	"github.com/tetratelabs/wazero/api"
)

// headersConstructor = Get("Headers").New() // http.Roundtrip && "fetch"
var headersConstructor = newJsVal(refHttpHeadersConstructor, "Headers")

type RoundTripperKey struct{}

func getRoundTripper(ctx context.Context) http.RoundTripper {
	if rt, ok := ctx.Value(RoundTripperKey{}).(http.RoundTripper); ok {
		return rt
	}
	return nil
}

// fetch is used to implement http.RoundTripper
//
// Reference in roundtrip_js.go init
//
//	jsFetchMissing = js.Global().Get("fetch").IsUndefined()
//
// In http.Transport RoundTrip, this returns a promise
//
//	fetchPromise := js.Global().Call("fetch", req.URL.String(), opt)
type fetch struct{}

// invoke implements jsFn.invoke
func (*fetch) invoke(ctx context.Context, _ api.Module, args ...interface{}) (interface{}, error) {
	rt := getRoundTripper(ctx)
	if rt == nil {
		panic("unexpected to reach here without roundtripper as property is nil checked")
	}
	url := args[0].(string)
	properties := args[1].(*object).properties
	req, err := http.NewRequestWithContext(ctx, properties["method"].(string), url, nil)
	if err != nil {
		return nil, err
	}
	// TODO: headers properties[headers]
	v := &fetchPromise{rt: rt, req: req}
	return v, nil
}

type fetchPromise struct {
	rt  http.RoundTripper
	req *http.Request
}

// call implements jsCall.call
func (p *fetchPromise) call(ctx context.Context, mod api.Module, this ref, method string, args ...interface{}) (interface{}, error) {
	if method == "then" {
		if res, err := p.rt.RoundTrip(p.req); err != nil {
			failure := args[1].(funcWrapper)
			// HTTP is at the GOOS=js abstraction, so we can return any error.
			return failure.invoke(ctx, mod, this, err)
		} else {
			success := args[0].(funcWrapper)
			return success.invoke(ctx, mod, this, &fetchResult{res: res})
		}
	}
	panic(fmt.Sprintf("TODO: fetchPromise.%s", method))
}

type fetchResult struct {
	res *http.Response
}

// get implements jsGet.get
func (s *fetchResult) get(ctx context.Context, propertyKey string) interface{} {
	switch propertyKey {
	case "headers":
		names := make([]string, 0, len(s.res.Header))
		for k := range s.res.Header {
			names = append(names, k)
		}
		// Sort names for consistent iteration
		sort.Strings(names)
		h := &headers{names: names, headers: s.res.Header}
		return h
	case "body":
		// return undefined as arrayPromise is more complicated than an array.
		return undefined
	case "status":
		return uint32(s.res.StatusCode)
	}
	panic(fmt.Sprintf("TODO: get fetchResult.%s", propertyKey))
}

// call implements jsCall.call
func (s *fetchResult) call(ctx context.Context, _ api.Module, this ref, method string, _ ...interface{}) (interface{}, error) {
	switch method {
	case "arrayBuffer":
		v := &arrayPromise{reader: s.res.Body}
		return v, nil
	}
	panic(fmt.Sprintf("TODO: call fetchResult.%s", method))
}

type headers struct {
	headers http.Header
	names   []string
	i       int
}

// get implements jsGet.get
func (h *headers) get(_ context.Context, propertyKey string) interface{} {
	switch propertyKey {
	case "done":
		return h.i == len(h.names)
	case "value":
		name := h.names[h.i]
		value := h.headers.Get(name)
		h.i++
		return &objectArray{[]interface{}{name, value}}
	}
	panic(fmt.Sprintf("TODO: get headers.%s", propertyKey))
}

// call implements jsCall.call
func (h *headers) call(_ context.Context, _ api.Module, this ref, method string, args ...interface{}) (interface{}, error) {
	switch method {
	case "entries":
		// Sort names for consistent iteration
		sort.Strings(h.names)
		return h, nil
	case "next":
		return h, nil
	case "append":
		name := textproto.CanonicalMIMEHeaderKey(args[0].(string))
		value := args[1].(string)
		h.names = append(h.names, name)
		h.headers.Add(name, value)
		return nil, nil
	}
	panic(fmt.Sprintf("TODO: call headers.%s", method))
}

type arrayPromise struct {
	reader io.ReadCloser
}

// call implements jsCall.call
func (p *arrayPromise) call(ctx context.Context, mod api.Module, this ref, method string, args ...interface{}) (interface{}, error) {
	switch method {
	case "then":
		defer p.reader.Close()
		if b, err := io.ReadAll(p.reader); err != nil {
			// HTTP is at the GOOS=js abstraction, so we can return any error.
			return args[1].(funcWrapper).invoke(ctx, mod, this, err)
		} else {
			return args[0].(funcWrapper).invoke(ctx, mod, this, &byteArray{b})
		}
	}
	panic(fmt.Sprintf("TODO: call arrayPromise.%s", method))
}
