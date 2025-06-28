package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
)

type internalProxy struct {
	rp     *httputil.ReverseProxy
	remote string
}

type Proxy struct {
	proxies map[string]internalProxy
	suren   int
}

type localServer struct {
	domain  string
	whereTo string
}

func NewProxy(servers []localServer) *Proxy {
	proxies := make(map[string]internalProxy)

	for _, server := range servers {
		rp := &httputil.ReverseProxy{
			Rewrite: func(req *httputil.ProxyRequest) {
				req.Out.URL.Scheme = "http"
				req.Out.URL.Host = server.whereTo
				req.Out.Host = server.whereTo

				// this is important to get real ip from SLdent
				req.SetXForwarded()
			},
			ModifyResponse: func(resp *http.Response) error {
				if strings.HasPrefix(resp.Request.URL.Path, "/_app/immutable/") {
					resp.Header["Cache-Control"] = []string{"public, max-age=31536000, immutable"}
				}
				return nil
			},
			ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
				if err != nil {
					log.Printf("Proxy error: %s", err.Error())
				}
			},
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
