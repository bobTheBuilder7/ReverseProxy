package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
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

	server := &http.Server{
		Handler: NewProxy(servers),
	}

	if app.dev {
		go func() {
			ln, err := net.Listen("tcp", ":"+httpPort)
			if err != nil {
				panic(err)
			}

			err = server.Serve(ln)
			if err != nil {
				log.Fatal(err)
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
			NextProtos:     []string{"h2", "http/1.1", "acme-tls/1"},
			ServerName:     "COBOL",
		}
		go func() {
			ln, err := tls.Listen("tcp", ":"+httpsPort, cfg)
			if err != nil {
				panic(err)
			}
			log.Printf("listening on %s", ln.Addr())
			err = server.Serve(ln)
			if err != nil {
				log.Fatal(err)
			}
		}()

		go func() {
			_ = http.ListenAndServe(":80", http.HandlerFunc(redirectToHTTPS))
		}()
	}

	select {
	case <-ctx.Done():
		err := server.Shutdown(ctx)
		if err != nil {
			return err
		}

		return nil
	}
}
