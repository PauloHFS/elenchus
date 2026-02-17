package main

import (
	"fmt"
	"os"

	"github.com/PauloHFS/goth/internal/cmd"
	"github.com/PauloHFS/goth/web/static/assets"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	if len(os.Args) < 2 {
		cmd.RunServer(assets.FS)
		return
	}

	switch os.Args[1] {
	case "server":
		cmd.RunServer(assets.FS)
	case "seed":
		cmd.RunSeed()
	case "migrate":
		cmd.RunMigrate()
	case "create-user":
		cmd.RunCreateUser()
	case "help":
		showHelp()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		showHelp()
		os.Exit(1)
	}
}

func showHelp() {
	fmt.Println("GOTH Stack - Single Binary Console")
	fmt.Println("Usage: ./goth [command] [args]")
	fmt.Println("\nAvailable commands:")
	fmt.Println("  server       Start the web server (default)")
	fmt.Println("  migrate      Run database migrations")
	fmt.Println("  seed         Run migrations and seed the database")
	fmt.Println("  create-user  Create a new user (args: <email> <password>)")
	fmt.Println("  help         Show this help message")
}
