package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

func NewHub(ctx context.Context, capacity int) *Hub {
	h := &Hub{
		mu:                  sync.RWMutex{},
		subscriptions:       make(map[string]map[string]Subscription),
		subscriptionHandler: make(chan Subscription),
		publishHandler:      make(chan string),
		client: http.Client{
			Timeout: time.Second * 5,
		},
	}

	h.subscriptions["/a/topic"] = make(map[string]Subscription)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case sub := <-h.subscriptionHandler:
				if err := h.HandleSubscription(sub); err != nil {
					log.Println(err)
				}
			}
		}
	}()

	for range capacity {
		go h.publishWorker(ctx)
	}

	return h
}

func (h *Hub) publishWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-h.publishHandler:
			h.publishToSubscribers(t)
		}
	}
}

func (h *Hub) publishToSubscribers(topic string) (err error) {
	if !h.HasTopic(topic) {
		return errors.New("topic not found")
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, sub := range h.subscriptions[topic] {
		if err := h.Send(Content{
			Subscription: sub,
			Data:         DataMessage,
		}); err != nil {
			log.Println(err)
		}
	}

	return
}

func (h *Hub) HasTopic(topic string) (ok bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	_, ok = h.subscriptions[topic]
	return
}

func (h *Hub) HandleSubscription(sub Subscription) (err error) {
	if err = h.verify(sub); err != nil {
		return
	}

	if !h.HasTopic(sub.Topic) {
		return errors.New("topic not found")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	switch sub.Mode {
	case ModeSubscribe:
		h.subscriptions[sub.Topic][sub.Callback] = sub
	case ModeUnsubscribe:
		delete(h.subscriptions[sub.Topic], sub.Callback)
	}

	return
}

func (h *Hub) verify(sub Subscription) (err error) {
	req, err := http.NewRequest(http.MethodGet, sub.Callback, nil)

	if err != nil {
		return
	}

	challenge := rand.Text()

	q := req.URL.Query()
	q.Add("hub.mode", string(sub.Mode))
	q.Add("hub.topic", sub.Topic)
	q.Add("hub.challenge", challenge)

	req.URL.RawQuery = q.Encode()

	resp, err := h.client.Do(req)

	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return errors.New("challenge failed")
	}

	respBody, err := io.ReadAll(resp.Body)

	if err != nil {
		return
	}

	if string(respBody) != challenge {
		return errors.New("challenge failed")
	}

	return
}

func (h *Hub) Publish(topic string) (err error) {
	if !h.HasTopic(topic) {
		return errors.New("topic not found")
	}

	h.publishHandler <- topic

	return
}

func (h *Hub) Send(data Content) (err error) {
	dataBuf := bytes.NewBufferString(data.Data)
	req, err := http.NewRequest(http.MethodPost, data.Subscription.Callback, dataBuf)

	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")

	if data.Subscription.Secret != "" {
		h := hmac.New(sha256.New, []byte(data.Subscription.Secret))
		if _, err = h.Write(dataBuf.Bytes()); err != nil {
			return
		}

		req.Header.Set("X-Hub-Signature", "sha256="+hex.EncodeToString(h.Sum(nil)))
	}

	resp, err := h.client.Do(req)

	if err != nil {
		return
	}

	defer resp.Body.Close()

	return
}
