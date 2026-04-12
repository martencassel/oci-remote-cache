package transport

import "net/http"

type RTFactory func(http.RoundTripper) http.RoundTripper

func NewTransportChain(base http.RoundTripper, fns ...RTFactory) http.RoundTripper {
	rt := base
	for i := len(fns) - 1; i >= 0; i-- {
		rt = fns[i](rt)
	}
	return rt
}
