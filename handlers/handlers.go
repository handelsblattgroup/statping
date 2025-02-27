package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"path"
	"sync"
	"time"

	"github.com/handelsblattgroup/statping/source"
	statpingErrors "github.com/handelsblattgroup/statping/types/errors"
	"github.com/handelsblattgroup/statping/utils"
)

const (
	cookieName = "statping_auth"

	timeout = time.Second * 30
)

var (
	jwtKey            []byte
	httpServer        *http.Server
	maintenanceServer *http.Server
	usingSSL          bool
	mainTmpl          = `{{define "main" }} {{ template "base" . }} {{ end }}`
	templates         = []string{"base.gohtml"}
	waitGroup         *sync.WaitGroup
)

func StopHTTPServer(err error) {
	log.Infoln("Stopping HTTP Server")
	// TODO: add graceful shutdown
	// err = maintenanceServer.Shutdown(context.Background())
	// if err != nil {
	// 	maintenanceLog.Fatal(errors.Wrapf(err, "shuting down maintenance server failed"))
	// }

	// err = httpServer.Shutdown(context.Background())
	// if err != nil {
	// 	maintenanceLog.Fatal(errors.Wrapf(err, "shuting down maintenance server failed"))
	// }
}

// RunHTTPServer will start a HTTP server on a specific IP and port
func RunHTTPServer() {
	waitGroup = new(sync.WaitGroup)

	maintenanceServerEnabled := utils.Params.GetBool("MAINTENANCE_SERVER_ENABLED")

	if maintenanceServerEnabled {
		waitGroup.Add(1)

		maintenanceIpAddress := utils.Params.GetString("MAINTENANCE_SERVER_IP")
		maintenancePort := utils.Params.GetString("MAINTENANCE_SERVER_PORT")
		maintenanceHost := fmt.Sprintf("%v:%v", maintenanceIpAddress, maintenancePort)
		maintenanceRouter = MaintenanceRouter()

		startMaintenanceServer(maintenanceHost)

		maintenanceLog.Infoln("Statping HTTP Maintenance Server running on http://" + maintenanceHost + basePath)
	}

	if utils.Params.GetBool("DISABLE_HTTP") {
		return
	}

	waitGroup.Add(1)

	ip := utils.Params.GetString("SERVER_IP")
	port := utils.Params.GetString("SERVER_PORT")
	host := fmt.Sprintf("%v:%v", ip, port)
	key := utils.FileExists(utils.Directory + "/server.key")
	cert := utils.FileExists(utils.Directory + "/server.crt")

	if key && cert {
		log.Infoln("server.cert and server.key was found in root directory! Starting in SSL mode.")
		log.Infoln(fmt.Sprintf("Statping Secure HTTPS Server running on https://%v:%v", ip, 443))
		usingSSL = true
	} else {
		log.Infoln("Statping HTTP Server running on http://" + host + basePath)
	}

	router = Router()
	resetCookies()

	if utils.Params.GetBool("LETSENCRYPT_ENABLE") {
		startLetsEncryptServer(ip)
	} else if usingSSL {
		startSSLServer(ip)
	} else {
		startServer(host)
	}

	waitGroup.Wait()
}

// IsReadAuthenticated will allow Read Only authentication for some routes
func IsReadAuthenticated(r *http.Request) bool {
	if ok := hasSetupEnv(); ok {
		return true
	}
	if ok := hasAPIQuery(r); ok {
		return true
	}
	if ok := hasAuthorizationHeader(r); ok {
		return true
	}
	_, err := getJwtToken(r)

	return err == nil
}

// IsFullAuthenticated returns true if the HTTP request is authenticated. You can set the environment variable GO_ENV=test
// to bypass the admin authenticate to the dashboard features.
func IsFullAuthenticated(r *http.Request) bool {
	if ok := hasSetupEnv(); ok {
		return true
	}
	if ok := hasAPIQuery(r); ok {
		return true
	}
	if ok := hasAuthorizationHeader(r); ok {
		return true
	}
	claim, err := getJwtToken(r)
	if err != nil {
		return false
	}
	return claim.Admin
}

// ScopeName will show private JSON fields in the API.
// It will return "admin" if request has valid admin authentication.
func ScopeName(r *http.Request) string {
	if ok := hasAPIQuery(r); ok {
		return "admin"
	}
	if ok := hasAuthorizationHeader(r); ok {
		return "admin"
	}
	claim, err := getJwtToken(r)
	if err != nil {
		return ""
	}
	if claim.Admin {
		return "admin"
	}
	return "user"
}

// IsAdmin returns true if the user session is an administrator
func IsAdmin(r *http.Request) bool {
	claim, err := getJwtToken(r)
	if err != nil {
		return false
	}
	return claim.Admin
}

// IsUser returns true if the user is registered
func IsUser(r *http.Request) bool {
	if ok := hasSetupEnv(); ok {
		return true
	}
	tk, err := getJwtToken(r)
	if err != nil {
		return false
	}
	if err := tk.Valid(); err != nil {
		return false
	}
	return true
}

func loadTemplate(w http.ResponseWriter, r *http.Request) (*template.Template, error) {
	var err error
	mainTemplate := template.New("main")
	mainTemplate, err = mainTemplate.Parse(mainTmpl)
	if err != nil {
		log.Errorln(err)
		return nil, err
	}
	mainTemplate.Funcs(handlerFuncs(w, r))
	// render all templates
	for _, temp := range templates {
		tmp, _ := source.TmplBox.String(temp)
		mainTemplate, err = mainTemplate.Parse(tmp)
		if err != nil {
			log.Errorln(err)
			return nil, err
		}
	}
	return mainTemplate, err
}

// ExecuteResponse will render a HTTP response for the front end user
func ExecuteResponse(w http.ResponseWriter, r *http.Request, file string, data interface{}, redirect interface{}) {
	if url, ok := redirect.(string); ok {
		http.Redirect(w, r, path.Join(basePath, url), http.StatusSeeOther)
		return
	}
	if usingSSL {
		w.Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
	}
	mainTemplate, err := loadTemplate(w, r)
	if err != nil {
		log.Errorln(err)
	}

	asset := file
	if source.UsingAssets(utils.Directory) {

		asset = utils.Directory + "/assets/" + file

		if _, err := mainTemplate.ParseFiles(asset); err != nil {
			log.Errorln(err)
		}
	} else {
		render, err := source.TmplBox.String(asset)
		if err != nil {
			log.Errorln(err)
		}
		// render the page requested
		if _, err := mainTemplate.Parse(render); err != nil {
			log.Errorln(err)
		}
	}
	// execute the template
	if err := mainTemplate.Execute(w, data); err != nil {
		log.Errorln(err)
	}
}

func returnJson(d interface{}, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if e, ok := d.(statpingErrors.Error); ok {
		w.WriteHeader(e.Status())
		json.NewEncoder(w).Encode(e)
		return
	}
	if e, ok := d.(error); ok {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(statpingErrors.New(e.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(d)
}

// error404Handler is a HTTP handler for 404 error pages
func error404Handler(w http.ResponseWriter, r *http.Request) {
	if usingSSL {
		w.Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
	}
	w.WriteHeader(http.StatusNotFound)
	ExecuteResponse(w, r, "base.gohtml", nil, nil)
}
