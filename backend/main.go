package main

import (
	"crypto/tls"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	httpPort := flag.Int("http-port", 8080, "HTTP server port")
	httpsPort := flag.Int("https-port", 8443, "HTTPS server port")
	certFile := flag.String("cert", "/var/cert/tls.crt", "TLS certificate file")
	keyFile := flag.String("key", "/var/cert/tls.key", "TLS key file")
	flag.Parse()

	static, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to create sub filesystem: %v", err)
	}

	handler := loggingMiddleware(http.FileServer(http.FS(static)))

	_, certErr := os.Stat(*certFile)
	_, keyErr := os.Stat(*keyFile)
	if certErr == nil && keyErr == nil {
		go func() {
			ln, err := net.Listen("tcp", fmt.Sprintf(":%d", *httpPort))
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("Listening on http://%s", ln.Addr())
			log.Fatal(http.Serve(ln, handler))
		}()

		cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			log.Fatalf("Failed to load TLS certificate: %v", err)
		}
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", *httpsPort))
		if err != nil {
			log.Fatal(err)
		}
		tlsLn := tls.NewListener(ln, &tls.Config{
			Certificates: []tls.Certificate{cert},
		})
		log.Printf("Listening on https://%s", ln.Addr())
		log.Fatal(http.Serve(tlsLn, handler))
	} else {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", *httpPort))
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("TLS certificate not found, listening on http://%s", ln.Addr())
		log.Fatal(http.Serve(ln, handler))
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
