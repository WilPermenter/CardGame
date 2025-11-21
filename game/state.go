package game

type Game struct {
    Players []*Player
    Turn    int
}

type Player struct {
    ID    int
    Hand  []int
    Life  int
}

var CurrentGame = NewGame()

func NewGame() *Game {
    return &Game{
        Players: []*Player{
            {ID: 1, Life: 20, Hand: []int{1, 2}},
            {ID: 2, Life: 20, Hand: []int{1, 2}},
        },
        Turn: 1,
    }
}
