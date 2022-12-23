package handlers

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"

	"github.com/foomo/simplecert"
	"github.com/foomo/tlsconfig"
	"github.com/handelsblattgroup/statping/utils"
	"github.com/pkg/errors"
)

func startMaintenanceServer(host string) {
	maintenanceServer = &http.Server{
		Addr:         host,
		WriteTimeout: timeout,
		ReadTimeout:  timeout,
		IdleTimeout:  timeout,
		Handler:      maintenanceRouter,
	}

	go func() {
		err := maintenanceServer.ListenAndServe()
		if err != nil {
			maintenanceLog.Fatal(errors.Wrapf(err, "starting maintenance server failed"))
		}

		waitGroup.Done()
	}()
}

func startServer(host string) {
	httpServer = &http.Server{
		Addr:         host,
		WriteTimeout: timeout,
		ReadTimeout:  timeout,
		IdleTimeout:  timeout,
		Handler:      router,
	}
	httpServer.SetKeepAlivesEnabled(false)

	go func() {
		err := httpServer.ListenAndServe()
		if err != nil {
			log.Fatal(errors.Wrapf(err, "starting server failed"))
		}

		waitGroup.Done()
	}()
}

func letsEncryptCert() (*tls.Config, error) {
	if !utils.FolderExists(utils.Directory + "/certs") {
		if err := utils.CreateDirectory(utils.Directory + "/certs"); err != nil {
			return nil, err
		}
	}

	cfg := simplecert.Default
	cfg.Domains = strings.Split(utils.Params.GetString("LETSENCRYPT_HOST"), ",")
	cfg.CacheDir = utils.Directory + "/certs"
	cfg.SSLEmail = utils.Params.GetString("LETSENCRYPT_EMAIL")
	cfg.Local = utils.Params.GetBool("LETSENCRYPT_LOCAL")
	cfg.WillRenewCertificate = func() {
		log.Infoln("LetsEncrypt renewing SSL Certificate for: ", utils.Params.GetString("LETSENCRYPT_HOST"))
	}
	cfg.DidRenewCertificate = func() {
		log.Infoln("LetsEncrypt renewed SSL Certificate for: ", utils.Params.GetString("LETSENCRYPT_HOST"))
		StopHTTPServer(nil)
		RunHTTPServer()
	}
	cfg.FailedToRenewCertificate = func(err error) {
		log.Errorln(err)
	}
	certReloader, err := simplecert.Init(cfg, func() {
		StopHTTPServer(nil)
	})
	if err != nil {
		log.Fatal("simplecert init failed: ", err)
		return nil, err
	}

	tlsconf := tlsconfig.NewServerTLSConfig(tlsconfig.TLSModeServerStrict)
	tlsconf.GetCertificate = certReloader.GetCertificateFunc()

	return tlsconf, nil
}

func startLetsEncryptServer(ip string) {
	log.Infoln("Starting LetEncrypt redirect server on port 80")
	go http.ListenAndServe(":80", http.HandlerFunc(simplecert.Redirect))

	cfg, err := letsEncryptCert()
	if err != nil {
		log.Fatal(errors.Wrapf(err, "loading LetsEncrypt certificate failed"))
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf("%v:%v", ip, 443),
		Handler:      router,
		TLSConfig:    cfg,
		WriteTimeout: timeout,
		ReadTimeout:  timeout,
		IdleTimeout:  timeout,
	}

	go func() {
		err := srv.ListenAndServeTLS("", "")
		if err != nil {
			log.Fatal(errors.Wrapf(err, "starting server failed"))
		}

		waitGroup.Done()
	}()
}

func startSSLServer(ip string) {
	cfg := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		},
	}
	srv := &http.Server{
		Addr:         fmt.Sprintf("%v:%v", ip, 443),
		Handler:      router,
		TLSConfig:    cfg,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
		WriteTimeout: timeout,
		ReadTimeout:  timeout,
		IdleTimeout:  timeout,
	}

	certFile := utils.Directory + "/server.crt"
	keyFile := utils.Directory + "/server.key"

	go func() {
		err := srv.ListenAndServeTLS(certFile, keyFile)
		if err != nil {
			log.Fatal(errors.Wrapf(err, "starting server failed"))
		}

		waitGroup.Done()
	}()
}
