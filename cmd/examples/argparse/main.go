package main

import (
	"fmt"
	"os"

	"github.com/tnotstar/go-minolas/pkg/cli/argparse"
)

func main() {
	parser := argparse.NewArgumentParser("deployer", "A sample deployment CLI tool.")

	// Global options
	verbose := parser.Bool("--verbose", argparse.Short("v"), argparse.Help("Enable verbose output"))
	region := parser.String("--region", argparse.Short("r"), argparse.Default("us-east-1"), argparse.Help("Target Region"))

	// Create subcommands
	appCmd := parser.NewCommand("app", "Manage applications")
	appName := appCmd.String("name", argparse.Required(), argparse.Help("Name of the application"))
	replicas := appCmd.Int("--replicas", argparse.Default(3), argparse.Help("Number of instances to deploy"))

	dbCmd := parser.NewCommand("db", "Manage database")
	action := dbCmd.String("action", argparse.Required(), argparse.Help("Action to perform (migrate, seed, rollback)"))
	dbHost := dbCmd.String("--host", argparse.Default("localhost"), argparse.Help("Database host address"))

	err := parser.Parse(os.Args[1:])
	if err != nil {
		if err == argparse.ErrHelp {
			fmt.Print(parser.Usage())
			os.Exit(0)
		}
		fmt.Printf("Error: %v\n\n", err)
		fmt.Print(parser.Usage())
		os.Exit(1)
	}

	fmt.Printf("Global config -> Verbose: %v, Region: %s\n", *verbose, *region)

	if appCmd.Invoked() {
		fmt.Println("--- App Command Executed ---")
		fmt.Printf("Deploying app '%s' with %d replicas.\n", *appName, *replicas)
	} else if dbCmd.Invoked() {
		fmt.Println("--- DB Command Executed ---")
		fmt.Printf("Running '%s' on database at %s.\n", *action, *dbHost)
	} else {
		fmt.Println("No subcommand invoked. Try running with 'app' or 'db'.")
	}
}
