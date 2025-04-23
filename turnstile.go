package turnstile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Router is a struct that represents a router in the configuration file.
type Router struct {
	Method string `yaml:"method"`
	Path   string `yaml:"path"`
	// HeaderKey is the key of the header to check for the token, if not provided, the form key will be used
	HeaderKey string `yaml:"headerkey"`
	// FormKey is the key of the form to check for the token, if not provided, the default value cf-turnstile-response will be used
	FormKey string `yaml:"formkey"`
}

func (r *Router) isMatch(req *http.Request) bool {
	if !strings.EqualFold(req.Method, r.Method) {
		return false
	}

	requestPath := strings.ToLower(req.URL.Path)

	routerParts := strings.Split(strings.Trim(r.Path, "/"), "/")
	requestParts := strings.Split(strings.Trim(requestPath, "/"), "/")

	if len(routerParts) != len(requestParts) {
		return false
	}

	for i := 0; i < len(routerParts); i++ {
		// Check if this part is a parameter (wrapped in {})
		if strings.HasPrefix(routerParts[i], "{") && strings.HasSuffix(routerParts[i], "}") {
			continue // Skip parameter comparison
		}
		// Otherwise, check for exact match
		if routerParts[i] != requestParts[i] {
			return false
		}
	}
	return true
}

func (t *Router) getToken(req *http.Request) (string, error) {
	if t.HeaderKey != "" {
		return req.Header.Get(t.HeaderKey), nil
	}
	formKey := "cf-turnstile-response"
	if t.FormKey != "" {
		formKey = t.FormKey
	}
	copyReq, err := copyRequest(req)
	if err != nil {
		return "", errors.New("failed to copy request")
	}
	err = copyReq.ParseForm()
	if err != nil {
		return "", errors.New("failed to parse form")
	}
	token := copyReq.Form.Get(formKey)
	if token == "" {
		return "", errors.New("no token provided")
	}
	return token, nil
}

func init() {
	log.SetOutput(os.Stdout)
}

// Config the plugin configuration.
type Config struct {
	TurnstileSecret string   `yaml:"turnstilesecret"`
	Routers         []Router `yaml:"routers"`
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{}
}

// Demo a Demo plugin.
type turnstile struct {
	next             http.Handler
	secret           string
	protectedRouters []Router
}

// New created a new Demo plugin.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if len(config.TurnstileSecret) == 0 {
		return nil, fmt.Errorf("turnstilesecret cannot be empty")
	}

	return &turnstile{
		next:             next,
		secret:           config.TurnstileSecret,
		protectedRouters: config.Routers,
	}, nil
}

// checks for a specific header in the response, extracts its value,
// sends a notification POST request, and logs the result.
func (a *turnstile) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	router, ok := a.isProtectedPath(req)
	if !ok {
		a.next.ServeHTTP(rw, req)
		return
	}

	token, err := router.getToken(req)
	if err != nil {
		errorHandler(rw, http.StatusBadRequest, err.Error())
		return
	}

	form := url.Values{}
	form.Add("secret", a.secret)
	form.Add("response", token)
	// create request with form data
	myreq, err := http.NewRequest("POST", "https://challenges.cloudflare.com/turnstile/v0/siteverify", strings.NewReader(form.Encode()))
	myreq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err != nil {
		errorHandler(rw, http.StatusInternalServerError, "Failed to create verification request")
		return
	}

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(myreq)
	if err != nil {
		errorHandler(rw, http.StatusInternalServerError, "Failed to verify token")
		return
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		errorHandler(rw, http.StatusInternalServerError, "Failed to read verification response")
		return
	}

	// Parse the response
	var turnstileResp turnstileResponse
	if err := json.Unmarshal(body, &turnstileResp); err != nil {
		errorHandler(rw, http.StatusInternalServerError, "Failed to parse verification response")
		return
	}
	// Check if verification was successful
	if !turnstileResp.Success {
		errorHandler(rw, http.StatusBadRequest, fmt.Sprintf("Verification failed: %s", turnstileResp.ErrorCodes))
		return
	}
	a.next.ServeHTTP(rw, req)

}

func errorHandler(rw http.ResponseWriter, code int, msg string) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(code)
	_ = json.NewEncoder(rw).Encode(map[string]string{"error": msg})
}

func (a *turnstile) isProtectedPath(req *http.Request) (*Router, bool) {
	for _, router := range a.protectedRouters {
		if router.isMatch(req) {
			return &router, true
		}
	}
	return nil, false
}

type turnstileResponse struct {
	Success     bool     `json:"success"`
	ErrorCodes  []string `json:"error-codes"`
	ChallengeTS string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
}

func copyRequest(req *http.Request) (*http.Request, error) {
	// Read the request body
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	// Restore the original request's body for further use
	err = req.Body.Close()
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Create a new request with the same body
	newReq, err := http.NewRequest(req.Method, req.URL.String(), bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	// Copy headers
	for name, values := range req.Header {
		newReq.Header[name] = values
	}

	return newReq, nil
}
