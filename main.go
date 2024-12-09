package main

import (
	"log"
	"log/slog"
	"net/http"

	"github.com/TaeKwonZeus/pf"
)

type PingResponse struct {
	Message string `json:"message"`
}

func pingHandler(w pf.ResponseWriter[PingResponse], r *pf.Request[struct{}]) error {
	return w.OK(PingResponse{"Pong!"})
}

func main() {
	r := pf.NewRouter()
	pf.Get(r, "/ping", pingHandler)

	pf.AddSwagger(r, "/swagger", &pf.SwaggerInfo{
		Title:   "Gotovo Server Gateway",
		Version: "v0.0.0.0.0.0.0.0.0.0.0.1",
	})

	port := "8080"

	slog.Info("starting server", "port", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
