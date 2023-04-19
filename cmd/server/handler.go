package main

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"sync/atomic"

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

var replymap syncx.Map[uint32, *alpacapi.WorkerReply]

var globalid uint32

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
		tk, err = alpacapi.ParseTokenString(token, teakey, sumtable)
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
		ID: atomic.AddUint32(&globalid, 1),
		Config: alpacapi.Config{
			Role:    role,
			Default: defl,
		},
		Message: msg,
	}
	if _, ok := replymap.LoadOrStore(req.ID, nil); ok {
		http.Error(w, "403 Forbidden: worker busy", http.StatusForbidden)
		return
	}
	go func() {
		rep, err := req.GetReply(workers[rand.Intn(len(workers))], int(buffersize), timeout, teakey, sumtable)
		if err != nil {
			logrus.Warnln("[reply] get reply err:", err)
			replymap.Delete(req.ID)
			return
		}
		replymap.Store(req.ID, &rep)
	}()
	_ = json.NewEncoder(w).Encode(&req)
}

func get(w http.ResponseWriter, r *http.Request) {
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
		tk, err = alpacapi.ParseTokenString(token, teakey, sumtable)
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
	idstr := r.URL.Query().Get("id")
	if idstr == "" {
		http.Error(w, "400 Bad Request: empty id", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(idstr)
	if idstr == "" {
		http.Error(w, "400 Bad Request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if rep, ok := replymap.Load(uint32(id)); ok {
		if rep == nil {
			http.Error(w, "403 Forbidden: worker busy", http.StatusForbidden)
			return
		}
		_ = json.NewEncoder(w).Encode(rep)
		replymap.Delete(uint32(id))
		return
	}
	http.Error(w, "400 Bad Request: invalid id", http.StatusBadRequest)
}
