package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/shirou/magi_link/internal/game"
)

func main() {
	ebiten.SetWindowSize(1280, 720)
	ebiten.SetWindowTitle("Magi Link")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	g := game.New()
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
