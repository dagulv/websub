package main

import (
	"net/http"
	"sync"
)

type Mode string

const (
	ModeSubscribe   Mode = "subscribe"
	ModeUnsubscribe Mode = "unsubscribe"
)

const DataMessage = `{"test":"test"}`

type Hub struct {
	mu             sync.RWMutex
	subscriptions  map[string]map[string]Subscription
	publishHandler chan Content
	client         http.Client
}

type Subscription struct {
	Callback string
	Mode     Mode
	Topic    string
	Secret   string
}

type Content struct {
	Subscription Subscription
	Data         string
}
