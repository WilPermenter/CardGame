package server

import (
    "net/http"
)

func NewRouter() *http.ServeMux {
    mux := http.NewServeMux()
    mux.HandleFunc("/ws", ServeWs)
    return mux
}
