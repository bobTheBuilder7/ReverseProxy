package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type internalProxy struct {
	rp     *httputil.ReverseProxy
	remote string
}

type Proxy struct {
	proxies map[string]internalProxy
}

type localServer struct {
	domain  string
	whereTo string
}

func NewProxy(servers []localServer) *Proxy {

	proxies := make(map[string]internalProxy)

	for _, server := range servers {
		u, err := url.Parse("http://" + server.whereTo)
		if err != nil {
			panic(err)
		}

		rp := httputil.NewSingleHostReverseProxy(u)
		rp.Director = func(r *http.Request) {
			r.URL.Scheme = "http"
			r.URL.Host = server.whereTo
			r.Host = server.whereTo
		}

		proxies[server.domain] = internalProxy{
			rp:     rp,
			remote: server.whereTo,
		}
	}

	return &Proxy{proxies: proxies}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Alt-Svc", "h3=\":443\"; ma=2592000")

	h, ok := p.proxies[r.Host]

	if !ok {
		_, err := fmt.Fprint(w, "no such proxy:", r.Host)
		if err != nil {
			panic(err)
		}

		return
	}

	h.rp.ServeHTTP(w, r)

}
