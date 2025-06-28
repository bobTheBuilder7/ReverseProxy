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
				//start := time.Now()
				//acceptsEncoding := resp.Request.Header.Get("Accept-Encoding")
				//contentEncoding := resp.Header.Get("Content-Encoding")
				//
				//if contentEncoding != "" {
				//	return nil
				//}
				//
				//b := GetBuffer()
				//defer PutBuffer(b)
				//var alg string
				//if strings.Contains(acceptsEncoding, "zstd") {
				//	alg = "zstd"
				//	enc, err := zstd.NewWriter(b, zstd.WithEncoderLevel(zstd.SpeedDefault))
				//	if err != nil {
				//		return err
				//	}
				//
				//	_, err = io.Copy(enc, resp.Body)
				//	if err != nil {
				//		return err
				//	}
				//
				//	if err = resp.Body.Close(); err != nil {
				//		return err
				//	}
				//
				//	if err = enc.Close(); err != nil {
				//		return err
				//	}
				//
				//} else if strings.Contains(acceptsEncoding, "br") {
				//	alg = "br"
				//	writer := brotli.NewWriterV2(b, 4)
				//
				//	_, err := io.Copy(writer, resp.Body)
				//	if err != nil {
				//		return err
				//	}
				//
				//	if err = resp.Body.Close(); err != nil {
				//		return err
				//	}
				//
				//	if err = writer.Close(); err != nil {
				//		return err
				//	}
				//
				//} else if strings.Contains(acceptsEncoding, "gzip") {
				//	alg = "gzip"
				//	writer, err := gzip.NewWriterLevel(b, 6)
				//	if err != nil {
				//		return err
				//	}
				//
				//	_, err = io.Copy(writer, resp.Body)
				//	if err != nil {
				//		return err
				//	}
				//
				//	if err = resp.Body.Close(); err != nil {
				//		return err
				//	}
				//
				//	if err = writer.Close(); err != nil {
				//		return err
				//	}
				//
				//}
				//
				//resp.Header.Set("Content-Encoding", alg)
				//resp.Header.Set("Content-Length", strconv.Itoa(b.Len()))
				//resp.ContentLength = int64(b.Len())
				//resp.Body = io.NopCloser(bytes.NewReader(b.Bytes()))
				//resp.Header.Set("Time-To-Compress", fmt.Sprintf("%dms", time.Since(start).Milliseconds()))
				//
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

	if strings.HasPrefix(r.URL.Path, "/_app/immutable/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}

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
