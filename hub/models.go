package main

import (
	"net/http"
)

type Mode string

const (
	ModeSubscribe   Mode = "subscribe"
	ModeUnsubscribe Mode = "unsubscribe"
)

const DataMessage = `{"test":"test"}`

type Hub struct {
	subscriptions       map[string]map[string]Subscription
	subscriptionHandler chan Subscription
	publishHandler      chan string
	client              http.Client
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
