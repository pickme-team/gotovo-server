package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/TaeKwonZeus/pf"
	"github.com/pickme-team/gotovo-server/messaging"
)

func failOnError(err error, msg string) {
	if err != nil {
		slog.Error(msg, "err", err)
		os.Exit(1)
	}
}

type PingResponse struct {
	Message string `json:"message"`
}

func pingHandler(w pf.ResponseWriter[PingResponse], r *pf.Request[struct{}]) error {
	return w.OK(PingResponse{"Pong!"})
}

func main() {
	broker, err := messaging.CreateBroker(os.Getenv("RABBITMQ_URL"))
	failOnError(err, "RabbitMQ connection failed")
	defer broker.Close()

	r := pf.NewRouter()
	pf.Get(r, "/ping", pingHandler)

	err = pf.AddSwagger(r, "/swagger", &pf.SwaggerInfo{
		Title:   "Gotovo Server Gateway",
		Version: "v0.0.0.0.0.0.0.0.0.0.0.1",
	})
	failOnError(err, "failed to add Swagger")

	port := "8080"

	slog.Info("starting server", "port", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
