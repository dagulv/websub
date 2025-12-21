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
		mu:             sync.RWMutex{},
		subscriptions:  make(map[string]map[string]Subscription),
		publishHandler: make(chan Content),
		client: http.Client{
			Timeout: time.Second * 5,
		},
	}

	h.subscriptions["/a/topic"] = make(map[string]Subscription)

	for range capacity {
		go h.publishWorker(ctx, h.publishHandler)
	}

	return h
}

func (h *Hub) publishWorker(ctx context.Context, contents <-chan Content) {
	for {
		select {
		case <-ctx.Done():
			return
		case content := <-contents:
			if err := h.Send(content); err != nil {
				// TODO: Handle client publish error
				log.Println(err)
			}
		}
	}
}

func (h *Hub) getSubscriptions(topic string) []Subscription {
	h.mu.RLock()
	defer h.mu.RUnlock()

	subs := make([]Subscription, len(h.subscriptions[topic]))

	for _, sub := range h.subscriptions[topic] {
		subs = append(subs, sub)
	}

	return subs
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

	data := DataMessage

	subs := h.getSubscriptions(topic)

	go func() {
		for _, sub := range subs {
			h.publishHandler <- Content{
				Subscription: sub,
				Data:         data,
			}
		}
	}()

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
