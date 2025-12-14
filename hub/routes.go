package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type Routes struct {
	HubService *Hub
}

func handleErr(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	if err = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}); err != nil {
		log.Println(err)
	}
}

func (r Routes) RegisterSubscriptionRoute(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.NotFound(w, req)
		return
	}

	if err := req.ParseForm(); err != nil {
		handleErr(w, err)
		return
	}

	sub := Subscription{
		Callback: req.Form.Get("hub.callback"),
		Mode:     Mode(req.Form.Get("hub.mode")),
		Topic:    req.Form.Get("hub.topic"),
		Secret:   req.Form.Get("hub.secret"),
	}
	log.Println(sub)
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(http.StatusAccepted)

	go func() {
		r.HubService.subscriptionHandler <- sub
	}()
}

type publishData struct {
	Topic string `json:"topic"`
}

func (r Routes) RegisterPublishRoute(w http.ResponseWriter, req *http.Request) {
	var data publishData
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		handleErr(w, err)
		return
	}

	if err := r.HubService.Publish(data.Topic); err != nil {
		handleErr(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(http.StatusAccepted)
}
