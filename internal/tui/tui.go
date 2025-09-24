package tui

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/rivo/tview"
)

// Post represents a simplified post for display in the TUI.
type Post struct {
	Title string
	URL   string
}

// StartTUI initializes and runs the terminal user interface
func StartTUI(posts []Post) {
	app := tview.NewApplication()

	list := tview.NewList()
	for _, post := range posts {
		list.AddItem(post.Title, post.URL, 0, nil)
	}

	list.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		fmt.Printf("Opening post: %s\n", secondaryText)
		if err := openBrowser(secondaryText); err != nil {
			log.Printf("Failed to open browser: %v", err)
		}
	})

	if err := app.SetRoot(list, true).Run(); err != nil {
		log.Fatalf("Error running TUI: %v", err)
	}
}

// openBrowser opens the given URL in the default web browser
func openBrowser(url string) error {
	cmd := exec.Command("xdg-open", url)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}
