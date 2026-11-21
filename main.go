package main

import (
	"log"
	"os"
)

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "build" && os.Args[1] != "build-artifacts" && os.Args[1] != "dev") {
		log.Fatal("usage: go run . [build|build-artifacts|dev]")
	}
	if os.Args[1] == "dev" {
		if err := loadDevEnv("."); err != nil {
			log.Fatal(err)
		}
		log.Fatal(serve(".", "127.0.0.1:8080"))
	}
	if os.Args[1] == "build-artifacts" {
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
		log.Print("build process started")
		if err := buildFromArtifacts("."); err != nil {
			log.Fatal(err)
		}
		return
	}
	files, err := build(".")
	if err == nil {
		err = writeSite(".", files)
	}
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %d files to dist/", len(files))
}
