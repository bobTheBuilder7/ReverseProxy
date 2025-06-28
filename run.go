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

//var greaseValues = map[uint16]bool{
//	0x0a0a: true, 0x1a1a: true,
//	0x2a2a: true, 0x3a3a: true,
//	0x4a4a: true, 0x5a5a: true,
//	0x6a6a: true, 0x7a7a: true,
//	0x8a8a: true, 0x9a9a: true,
//	0xaaaa: true, 0xbaba: true,
//	0xcaca: true, 0xdada: true,
//	0xeaea: true, 0xfafa: true,
//}
//
//func calculateJA3(hello *tls.ClientHelloInfo) string {
//	var (
//		maxPossibleBufferLength = (5+1)*len(hello.CipherSuites) +
//			(5+1)*len(hello.Extensions) +
//			(5+1)*len(hello.SupportedCurves) +
//			(3+1)*len(hello.SupportedPoints)
//
//		buffer       = make([]byte, 0, maxPossibleBufferLength)
//		sepValueByte = byte(45)
//		sepFieldByte = byte(44)
//	)
//
//	lastElem := len(hello.CipherSuites) - 1
//	if len(hello.CipherSuites) > 1 {
//		for _, e := range hello.CipherSuites[:lastElem] {
//			// filter GREASE values
//			if !greaseValues[e] {
//				buffer = strconv.AppendInt(buffer, int64(e), 10)
//				buffer = append(buffer, sepValueByte)
//			}
//		}
//	}
//	// append last element if cipher suites are not empty
//	if lastElem != -1 {
//		// filter GREASE values
//		if !greaseValues[hello.CipherSuites[lastElem]] {
//			buffer = strconv.AppendInt(buffer, int64(hello.CipherSuites[lastElem]), 10)
//		}
//	}
//	buffer = bytes.TrimSuffix(buffer, []byte{sepValueByte})
//	buffer = append(buffer, sepFieldByte)
//
//	/*
//	 *	Extensions
//	 */
//
//	slices.Sort(hello.Extensions)
//
//	// collect extensions
//	lastElem = len(hello.Extensions) - 1
//	if len(hello.Extensions) > 1 {
//		for _, e := range hello.Extensions[:lastElem] {
//			// filter GREASE values
//			if !greaseValues[e] && e != 41 {
//				buffer = strconv.AppendInt(buffer, int64(e), 10)
//				buffer = append(buffer, sepValueByte)
//			}
//		}
//	}
//	// append last element if extensions are not empty
//	if lastElem != -1 {
//		// filter GREASE values
//		if !greaseValues[hello.Extensions[lastElem]] {
//			buffer = strconv.AppendInt(buffer, int64(hello.Extensions[lastElem]), 10)
//		}
//	}
//	buffer = bytes.TrimSuffix(buffer, []byte{sepValueByte})
//	buffer = append(buffer, sepFieldByte)
//
//	/*
//	 *	Supported Groups
//	 */
//
//	// collect supported groups
//	lastElem = len(hello.SupportedCurves) - 1
//	if len(hello.SupportedCurves) > 1 {
//		for _, e := range hello.SupportedCurves[:lastElem] {
//			// filter GREASE values
//			if !greaseValues[uint16(e)] {
//				buffer = strconv.AppendInt(buffer, int64(e), 10)
//				buffer = append(buffer, sepValueByte)
//			}
//		}
//	}
//	// append last element if supported groups are not empty
//	if lastElem != -1 {
//		// filter GREASE values
//		if !greaseValues[uint16(hello.SupportedCurves[lastElem])] {
//			buffer = strconv.AppendInt(buffer, int64(hello.SupportedCurves[lastElem]), 10)
//		}
//	}
//	buffer = bytes.TrimSuffix(buffer, []byte{sepValueByte})
//	buffer = append(buffer, sepFieldByte)
//
//	/*
//	 *	Supported Points
//	 */
//
//	// collect supported points
//	lastElem = len(hello.SupportedPoints) - 1
//	if len(hello.SupportedPoints) > 1 {
//		for _, e := range hello.SupportedPoints[:lastElem] {
//			buffer = strconv.AppendInt(buffer, int64(e), 10)
//			buffer = append(buffer, sepValueByte)
//		}
//	}
//	// append last element if supported points are not empty
//	if lastElem != -1 {
//		buffer = strconv.AppendInt(buffer, int64(hello.SupportedPoints[lastElem]), 10)
//	}
//
//	return hex.EncodeToString(buffer)
//}

func (app *application) run(ctx context.Context, httpsPort string) error {
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
			cer, err := tls.LoadX509KeyPair("./example.com+6.pem", "./example.com+6-key.pem")
			if err != nil {
				log.Println(err)
				return
			}

			cfg := &tls.Config{
				GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
					//cache.Set(info.Conn.RemoteAddr().String(), calculateJA3(info))
					return &cer, nil
				},

				NextProtos: []string{"h2", "http/1.1", "acme-tls/1"},
				ServerName: "COBOL",
			}
			ln, err := tls.Listen("tcp", ":"+httpsPort, cfg)
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
			GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
				//cache.Set(info.Conn.RemoteAddr().String(), calculateJA3(info))
				return m.GetCertificate(info)
			},
			NextProtos: []string{"h3", "h2", "http/1.1", "acme-tls/1"},
			ServerName: "COBOL",
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
