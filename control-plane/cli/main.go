package main

import (
	"fmt"
	"os"
)

// Prints CLI usage when user runs tinyaws with no or wrong args.
func printUsage() {
	fmt.Println(`tiny-aws CLI

Usage:
  tinyaws node list [--role compute|storage] [--healthy-only]
  tinyaws object put <key> [--data text] [--file path]
  tinyaws object get <key>
  tinyaws job submit "<command>" [--instance i-1]
  tinyaws job status <job-id> [--wait]
  tinyaws instance launch
  tinyaws instance list
  tinyaws instance terminate <id>

  tinyaws bucket create <name>
  tinyaws bucket list
  tinyaws object put <key> --bucket <name> [--data text] [--file path]
  tinyaws object get <key> --bucket <name>

  tinyaws deploy <dir> [--instance i-1] [--wait]

  tinyaws auth set-key <key> [--role admin|readonly]
  tinyaws auth whoami

Environment:
  REGISTRY_URL      default http://127.0.0.1:9000
  SCHEDULER_URL     default http://127.0.0.1:9001
  OBJECT_STORE_URL  default http://127.0.0.1:7001
  TINYAWS_API_KEY   optional bearer token`)
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
	case "bucket":
		runBucket(os.Args[2:])
	case "deploy":
		runDeploy(os.Args[2:])
	case "auth":
		runAuth(os.Args[2:])
	case "storage":
		runStorage(os.Args[2:])
	case "queue":
		runQueue(os.Args[2:])
	case "lb":
		runLB(os.Args[2:])
	case "vpc":
		runVPC(os.Args[2:])
	case "subnet":
		runSubnet(os.Args[2:])
	case "sg":
		runSG(os.Args[2:])
	case "lambda":
		runLambda(os.Args[2:])
	default:
		printUsage()
		os.Exit(1)
	}
}
