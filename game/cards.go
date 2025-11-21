package game

type Card struct {
    ID   int
    Name string
}

var CardDB = map[int]Card{
    1: {ID: 1, Name: "Firebolt"},
    2: {ID: 2, Name: "Goblin"},
}
