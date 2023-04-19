package main

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"net/url"

	"github.com/RomiChan/syncx"
	"github.com/sirupsen/logrus"

	"github.com/fumiama/alpacapi"
)

// isMethod check if the method meets the requirement
// and response 405 Method Not Allowed if not matched
func isMethod(m string, w http.ResponseWriter, r *http.Request) bool {
	logrus.Infoln("[isMethod] accept", r.RemoteAddr, r.Method, r.URL)
	if r.Method != m {
		http.Error(w, "405 Method Not Allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

var tokenmap syncx.Map[string, *alpacapi.Token]

func reply(w http.ResponseWriter, r *http.Request) {
	if !isMethod("GET", w, r) {
		return
	}
	token := r.Header.Get("Authorization")
	if token == "" {
		http.Error(w, "400 Bad Request: empty token", http.StatusBadRequest)
		return
	}
	tk, _ := tokenmap.Load(token)
	var err error
	if tk == nil {
		tk, err = alpacapi.ParseTokenString(token)
		if err != nil {
			http.Error(w, "400 Bad Request: "+err.Error(), http.StatusBadRequest)
			return
		}
		if !tk.IsValid() {
			http.Error(w, "400 Bad Request: invalid token", http.StatusBadRequest)
			return
		}
		tokenmap.Store(token, tk)
	} else {
		if !tk.IsValid() {
			http.Error(w, "400 Bad Request: invalid token", http.StatusBadRequest)
			return
		}
	}
	msg := r.URL.Query().Get("msg")
	if msg == "" {
		http.Error(w, "400 Bad Request: empty msg", http.StatusBadRequest)
		return
	}
	role := r.URL.Query().Get("role")
	if role == "" {
		role = "JK"
	} else {
		role, err = url.QueryUnescape(role)
		if err != nil {
			http.Error(w, "400 Bad Request: invalid role: "+err.Error(), http.StatusBadRequest)
			return
		}
	}
	defl := r.URL.Query().Get("default")
	if defl == "" {
		defl = "6"
	} else {
		defl, err = url.QueryUnescape(defl)
		if err != nil {
			http.Error(w, "400 Bad Request: invalid default: "+err.Error(), http.StatusBadRequest)
			return
		}
	}
	msg, err = url.QueryUnescape(msg)
	if err != nil {
		http.Error(w, "400 Bad Request: invalid msg: "+err.Error(), http.StatusBadRequest)
		return
	}
	req := alpacapi.WorkerRequest{
		Config: alpacapi.Config{
			Role:    role,
			Default: defl,
		},
		Message: msg,
	}
	rep, err := req.GetReply(workers[rand.Intn(len(workers))], int(buffersize), timeout, teakey, sumtable)
	if err != nil {
		logrus.Warnln("[reply] get reply err:", err)
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		return
	}
	if rep.IsPending {
		logrus.Warnln("[reply] worker response err: is pending")
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(&rep)
}
