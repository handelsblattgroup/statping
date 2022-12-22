package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/handelsblattgroup/statping/utils"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	_ "github.com/handelsblattgroup/statping/types/metrics"
)

var (
	maintenanceRouter *mux.Router
	maintenanceLog    = utils.Log.WithField("type", "maintenance-handlers")
)

// Router returns all of the routes used in Statping.
// Server will use static assets if the 'assets' directory is found in the root directory.
func MaintenanceRouter() *mux.Router {
	r := mux.NewRouter().StrictSlash(true)

	// API Generic Routes
	r.Handle("/metrics", promhttp.Handler())
	r.Handle("/health", http.HandlerFunc(healthCheckHandler))

	return r
}
