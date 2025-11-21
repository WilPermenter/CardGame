package main

import (
    "log"
    "net/http"

    "card-game/server"
)

func main() {
    router := server.NewRouter()

    log.Println("Server running on :8080")
    err := http.ListenAndServe(":8080", router)
    if err != nil {
        log.Fatal(err)
    }
}
