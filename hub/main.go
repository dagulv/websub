package main

import (
	"context"
	"log"
	"net/http"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mux := http.NewServeMux()

	routes := Routes{
		HubService: NewHub(ctx, 10),
	}

	mux.HandleFunc("/", routes.RegisterSubscriptionRoute)
	mux.HandleFunc("/publish", routes.RegisterPublishRoute)

	go func() {
		defer cancel()

		if err := http.ListenAndServe(":8080", mux); err != nil {
			log.Fatal(err)
			return
		}
	}()

	log.Println("starting http server")

	<-ctx.Done()
}
