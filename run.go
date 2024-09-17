package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/CAFxX/httpcompression"
	"github.com/CAFxX/httpcompression/contrib/andybalholm/brotli"
	"github.com/CAFxX/httpcompression/contrib/klauspost/gzip"
	"github.com/CAFxX/httpcompression/contrib/klauspost/zstd"
	kpzstd "github.com/klauspost/compress/zstd"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"golang.org/x/crypto/acme/autocert"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

type arrayFlags []string

func (i *arrayFlags) String() string {
	return ""
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var myFlags arrayFlags

func init() {
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}
	rLimit.Cur = rLimit.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func (app *application) run(ctx context.Context, httpPort, httpsPort string) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
	defer cancel()

	flag.Var(&myFlags, "url", "<domain>:<port>")
	flag.Parse()

	if len(myFlags) == 0 {
		fmt.Println("please specify at least one URL to proxy")
		os.Exit(0)
	}

	var servers []localServer
	var domains []string

	for _, something := range myFlags {
		domain, whereTo := strings.Split(something, "::")[0], strings.Split(something, "::")[1]
		servers = append(servers, localServer{
			domain:  domain,
			whereTo: whereTo,
		})
		domains = append(domains, domain)
	}

	zsEnc, err := zstd.New(kpzstd.WithEncoderLevel(kpzstd.SpeedBetterCompression))
	if err != nil {
		panic(err)
	}

	brEnc, err := brotli.New(brotli.Options{
		Quality: 4,
		LGWin:   0,
	})
	if err != nil {
		panic(err)
	}

	gzEnc, err := gzip.New(gzip.Options{Level: 6})
	if err != nil {
		panic(err)
	}

	compress, err := httpcompression.Adapter(
		httpcompression.Compressor(zstd.Encoding, 2, zsEnc),
		httpcompression.Compressor(brotli.Encoding, 1, brEnc),
		httpcompression.Compressor(gzip.Encoding, 0, gzEnc),
		httpcompression.Prefer(httpcompression.PreferServer),
		httpcompression.MinSize(200),
		httpcompression.ContentTypes([]string{
			"image/jpeg",
			"image/gif",
			"image/png",
		}, true),
	)
	if err != nil {
		panic(err)
	}

	serverH3 := http3.Server{
		Handler: compress(NewProxy(servers)),
	}

	serverH2 := http.Server{
		Handler: compress(NewProxy(servers)),
	}

	if app.dev {
		go func() {
			ln, err := net.Listen("tcp", ":"+httpPort)
			if err != nil {
				panic(err)
			}

			err = serverH2.Serve(ln)
			if err != nil {
				log.Println(err.Error())
			}
		}()
	} else {
		m := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(domains...),
			Cache:      autocert.DirCache("/.certs"),
		}
		cfg := &tls.Config{
			GetCertificate: m.GetCertificate,
			NextProtos:     []string{"h3", "h2", "http/1.1", "acme-tls/1"},
			ServerName:     "COBOL",
		}

		go func() {
			ln, err := tls.Listen("tcp", ":"+httpsPort, cfg)
			if err != nil {
				panic(err)
			}

			err = serverH2.Serve(ln)
			if err != nil {
				log.Println(err.Error())
			}
		}()

		go func() {
			udpConn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: 443})
			tr := quic.Transport{
				Conn: udpConn,
			}
			tlsConf := http3.ConfigureTLSConfig(cfg)
			quicConf := &quic.Config{Allow0RTT: true, EnableDatagrams: true}
			ln, _ := tr.ListenEarly(tlsConf, quicConf)
			err = serverH3.ServeListener(ln)
			if err != nil {
				log.Println(err.Error())
			}
		}()

		// http -> https redirect
		go func() {
			_ = http.ListenAndServe(":80", http.HandlerFunc(redirectToHTTPS))
		}()
	}

	log.Println("Starting servers...")

	select {
	case <-ctx.Done():
		err := serverH2.Shutdown(context.Background())
		if err != nil {
			log.Println(err.Error())
			return err
		}

		return ctx.Err()
	}
}
