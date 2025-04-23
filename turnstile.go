package turnstile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type Router struct {
	Method string `yaml:"method"`
	Path   string `yaml:"path"`
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
	protectedRouters map[string]bool
}

// New created a new Demo plugin.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if len(config.TurnstileSecret) == 0 {
		return nil, fmt.Errorf("turnstilesecret cannot be empty")
	}
	protectedRouters := make(map[string]bool)
	for _, router := range config.Routers {
		var buf strings.Builder
		buf.WriteString(strings.ToLower(router.Method))
		buf.WriteString(":")
		buf.WriteString(strings.ToLower(router.Path))
		protectedRouters[buf.String()] = true
	}
	return &turnstile{
		next:             next,
		secret:           config.TurnstileSecret,
		protectedRouters: protectedRouters,
	}, nil
}

// checks for a specific header in the response, extracts its value,
// sends a notification POST request, and logs the result.
func (a *turnstile) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if !a.isProtectedPath(req) {
		a.next.ServeHTTP(rw, req)
		return
	}
	copyReq, err := copyRequest(req)
	if err != nil {
		errorHandler(rw, http.StatusInternalServerError, "Failed to copy request")
		return
	}
	err = req.ParseForm()
	if err != nil {
		errorHandler(rw, http.StatusBadRequest, "Failed to parse form")
		return
	}
	token := req.Form.Get("cf-turnstile-response")
	if token == "" {
		errorHandler(rw, http.StatusBadRequest, "No token provided")
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
	a.next.ServeHTTP(rw, copyReq)

}

func errorHandler(rw http.ResponseWriter, code int, msg string) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(code)
	_ = json.NewEncoder(rw).Encode(map[string]string{"error": msg})
}

func (a *turnstile) isProtectedPath(req *http.Request) bool {
	router := strings.ToLower(req.Method) + ":" + strings.ToLower(req.URL.Path)
	return a.protectedRouters[router]
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
