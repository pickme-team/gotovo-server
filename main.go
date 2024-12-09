package main

import (
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
	http.ListenAndServe(":8080", r)
}
