package server

type Hub struct {
    connections map[*Connection]bool
}

var GameHub = &Hub{
    connections: make(map[*Connection]bool),
}
