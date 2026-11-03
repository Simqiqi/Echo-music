package main

import (
	"log"

	"github.com/simqiqi/simqiqi-music/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
