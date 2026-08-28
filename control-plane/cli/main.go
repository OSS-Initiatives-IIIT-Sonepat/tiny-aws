package main

import (
	"fmt"
	"os"
)

// Prints CLI usage when user runs tinyaws with no or wrong args.
func printUsage() {
	fmt.Println(`tiny-aws CLI

Usage:
  tinyaws node list [--role compute|storage]
  tinyaws object put <key> [--data text] [--file path]
  tinyaws object get <key>
  tinyaws job submit "<command>"
  tinyaws job status <job-id> [--wait]
  tinyaws instance launch
  tinyaws instance list
  tinyaws instance terminate <id>

Environment:
  REGISTRY_URL      default http://127.0.0.1:9000
  SCHEDULER_URL     default http://127.0.0.1:9001
  OBJECT_STORE_URL  default http://127.0.0.1:7001`)
}

// Entry point — parses subcommands and delegates.
func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "node":
		runNode(os.Args[2:])
	case "object":
		runObject(os.Args[2:])
	case "job":
		runJob(os.Args[2:])
	case "instance":
		runInstance(os.Args[2:])
	default:
		printUsage()
		os.Exit(1)
	}
}
