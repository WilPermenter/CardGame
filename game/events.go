package game

type Event struct {
    Type string                 `json:"type"`
    Data map[string]interface{} `json:"data"`
}
