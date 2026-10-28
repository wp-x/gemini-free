package app

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strings"
)

type Control struct {
	Controller *Controller
	Files      fs.FS
	Token      string
	Host       string
}

func NewControl(controller *Controller, files fs.FS) *Control {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		panic(err)
	}
	return &Control{Controller: controller, Files: files, Token: hex.EncodeToString(token[:])}
}

func (c *Control) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if r.Host != c.Host {
		writeError(w, http.StatusForbidden, fmt.Errorf("invalid management host"))
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/control/") {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'")
		http.FileServerFS(c.Files).ServeHTTP(w, r)
		return
	}
	token := r.Header.Get("X-Desktop-Token")
	if subtle.ConstantTimeCompare([]byte(token), []byte(c.Token)) != 1 {
		writeError(w, http.StatusUnauthorized, fmt.Errorf("invalid desktop token"))
		return
	}
	c.dispatch(w, r)
}

func (c *Control) dispatch(w http.ResponseWriter, r *http.Request) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /control/state", c.state)
	mux.HandleFunc("POST /control/config", c.configure)
	mux.HandleFunc("POST /control/service", c.service)
	mux.HandleFunc("POST /control/accounts", c.addAccount)
	mux.HandleFunc("POST /control/import-legacy", c.importLegacy)
	mux.HandleFunc("DELETE /control/accounts/{id}", c.deleteAccount)
	mux.HandleFunc("POST /control/accounts/{id}/sync", c.syncAccountParameters)
	mux.HandleFunc("POST /control/accounts/{id}/session", c.replaceAccountSession)
	mux.HandleFunc("POST /control/probe", c.probe)
	mux.ServeHTTP(w, r)
}

func (c *Control) state(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, c.Controller.State())
}

func (c *Control) configure(w http.ResponseWriter, r *http.Request) {
	var update SettingsUpdate
	if err := decodeBody(r, &update); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	c.finish(w, c.Controller.Configure(update))
}

func (c *Control) service(w http.ResponseWriter, r *http.Request) {
	var command struct {
		Action string `json:"action"`
	}
	if err := decodeBody(r, &command); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	switch command.Action {
	case "start":
		c.finish(w, c.Controller.Start())
	case "stop":
		c.finish(w, c.Controller.Stop())
	default:
		writeError(w, http.StatusBadRequest, fmt.Errorf("unknown service action"))
	}
}

func (c *Control) addAccount(w http.ResponseWriter, r *http.Request) {
	var input AccountInput
	if err := decodeBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	c.finish(w, c.Controller.AddAccount(input))
}

func (c *Control) deleteAccount(w http.ResponseWriter, r *http.Request) {
	c.finish(w, c.Controller.DeleteAccount(r.PathValue("id")))
}

func (c *Control) replaceAccountSession(w http.ResponseWriter, r *http.Request) {
	var input AccountInput
	if err := decodeBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	c.finish(w, c.Controller.ReplaceAccountSession(r.PathValue("id"), input))
}

func (c *Control) syncAccountParameters(w http.ResponseWriter, r *http.Request) {
	c.finish(w, c.Controller.SyncAccountParameters(r.Context(), r.PathValue("id")))
}

func (c *Control) finish(w http.ResponseWriter, err error) {
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, c.Controller.State())
}

func decodeBody(r *http.Request, value any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("request must contain exactly one JSON value")
	}
	return nil
}

func (c *Control) importLegacy(w http.ResponseWriter, _ *http.Request) {
	c.finish(w, c.Controller.ImportLegacy())
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		return
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	writeJSON(w, map[string]string{"error": err.Error()})
}

func (c *Control) Listen() (net.Listener, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	c.Host = listener.Addr().String()
	return listener, nil
}
