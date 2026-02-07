// This file contains the health check endpoint handler.
//   - GET / — Returns a simple health check response
package handlers

import (
	"net/http"
)

// HealthCheck handles GET / — a simple endpoint that confirms the API
// is running. Health check endpoints are standard practice in web services;
// they're used by load balancers and monitoring tools to verify the service
// is alive.
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeSuccess(w, http.StatusOK, map[string]string{
		"status":  "healthy",
		"service": "tinder-claude",
	}, nil)
}
