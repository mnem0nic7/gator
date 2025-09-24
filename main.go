package main

import (
	"fmt"
	"os"

	"gator/internal/config"
)

// state struct holds a pointer to a config
type state struct {
	cfg *config.Config
}

// command represents a parsed CLI command
type command struct {
	name string
	args []string
}

// commands struct holds all the commands the CLI can handle
type commands struct {
	handlers map[string]func(*state, command) error
}

// run method runs a given command with the provided state if it exists
func (c *commands) run(s *state, cmd command) error {
	handler, exists := c.handlers[cmd.name]
	if !exists {
		return fmt.Errorf("unknown command: %s", cmd.name)
	}
	return handler(s, cmd)
}

// register method registers a new handler function for a command name
func (c *commands) register(name string, f func(*state, command) error) {
	c.handlers[name] = f
}

// handlerLogin handles the login command
func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("usage: %s <username>", cmd.name)
	}
	
	username := cmd.args[0]
	err := s.cfg.SetUser(username)
	if err != nil {
		return fmt.Errorf("couldn't set current user: %w", err)
	}
	
	fmt.Printf("User has been set to: %s\n", username)
	return nil
}

func main() {
	// Read the config file
	cfg, err := config.Read()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
		os.Exit(1)
	}

	// Create state with config
	programState := &state{cfg: &cfg}

	// Create commands struct with initialized map
	cmds := &commands{
		handlers: make(map[string]func(*state, command) error),
	}

	// Register the login handler
	cmds.register("login", handlerLogin)

	// Get command-line arguments
	args := os.Args
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [args...]\n", args[0])
		os.Exit(1)
	}

	// Parse command name and arguments
	cmdName := args[1]
	cmdArgs := []string{}
	if len(args) > 2 {
		cmdArgs = args[2:]
	}

	// Create command instance
	cmd := command{
		name: cmdName,
		args: cmdArgs,
	}

	// Run the command
	err = cmds.run(programState, cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}