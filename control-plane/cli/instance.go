package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Handles: tinyaws instance launch|list|terminate|info ...
func runInstance(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: tinyaws instance launch [--type nano|micro|small|medium|large] [--volume host:container]...")
		fmt.Println("       tinyaws instance list")
		fmt.Println("       tinyaws instance terminate <id>")
		fmt.Println("       tinyaws instance info <id>")
		fmt.Println("       tinyaws instance shell <id>")
		os.Exit(1)
	}

	switch args[0] {
	case "launch":
		runInstanceLaunch(args[1:])
	case "list":
		runInstanceList()
	case "terminate":
		if len(args) < 2 {
			fmt.Println("usage: tinyaws instance terminate <id>")
			os.Exit(1)
		}
		runInstanceTerminate(args[1])
	case "info":
		if len(args) < 2 {
			fmt.Println("usage: tinyaws instance info <id>")
			os.Exit(1)
		}
		runInstanceInfo(args[1])
	case "shell":
		if len(args) < 2 {
			fmt.Println("usage: tinyaws instance shell <id>")
			os.Exit(1)
		}
		runInstanceShell(args[1])
	default:
		fmt.Println("usage: tinyaws instance launch|list|terminate|info|shell ...")
		os.Exit(1)
	}
}

// POST /instances — launch an instance. Optional --type and --volume flags.
func runInstanceLaunch(args []string) {
	instanceType := "small"
	var volumes []string
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--type" {
			instanceType = args[i+1]
			i++
		} else if args[i] == "--volume" {
			volumes = append(volumes, args[i+1])
			i++
		}
	}

	body := map[string]any{"instance_type": instanceType}
	if len(volumes) > 0 {
		body["volumes"] = volumes
	}
	payload, _ := json.Marshal(body)
	resp, err := httpPost(registryURL()+"/instances", "application/json",
		bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		fmt.Fprintf(os.Stderr, "launch failed %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	fmt.Println(string(body))
}

// GET /instances — list all instances.
func runInstanceList() {
	resp, err := httpGet(registryURL() + "/instances")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if resp.StatusCode >= 400 {
		fmt.Fprintf(os.Stderr, "list failed %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	var list []map[string]any
	if err := json.Unmarshal(body, &list); err != nil {
		fmt.Fprintf(os.Stderr, "decode: %v\n", err)
		os.Exit(1)
	}

	for _, inst := range list {
		fmt.Printf("%s node=%s status=%s\n", inst["id"], inst["node_id"], inst["status"])
	}
}

// DELETE /instances/{id} — terminate instance.
func runInstanceTerminate(id string) {
	req, err := http.NewRequest(http.MethodDelete, registryURL()+"/instances/"+id, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	resp, err := httpDo(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "terminate failed %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	fmt.Println("terminated", id)
}

// GET /instances/{id} — show instance node, status, and workspace path.
func runInstanceInfo(id string) {
	resp, err := httpGet(registryURL() + "/instances/" + id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		fmt.Fprintf(os.Stderr, "info failed %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	var inst map[string]any
	if err := json.Unmarshal(body, &inst); err != nil {
		fmt.Fprintf(os.Stderr, "decode: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("id:            %s\n", inst["id"])
	fmt.Printf("node:          %s\n", inst["node_id"])
	fmt.Printf("status:        %s\n", inst["status"])
	fmt.Printf("instance_type: %s\n", inst["instance_type"])
	fmt.Printf("cpu_limit:     %s\n", inst["cpu_limit"])
	fmt.Printf("mem_limit_mb:  %v\n", inst["mem_limit_mb"])
	fmt.Printf("workspace:     /var/lib/tinyaws/instances/%s\n", inst["id"])
}

// runInstanceShell drops into an interactive shell inside the instance container.
func runInstanceShell(id string) {
	// try machinectl shell first (interactive)
	cmd := exec.Command("sudo", "machinectl", "shell", id)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// fallback: print manual instructions
		fmt.Printf("shell failed: %v\n", err)
		fmt.Printf("\nRun manually on the agent machine:\n")
		fmt.Printf("  sudo machinectl shell %s\n", id)
	}
}

// Handles: tinyaws exec <instance-id> -- <command...>
// Submits a job to run inside the specified instance and waits for the result.
func runExec(args []string) {
	if len(args) < 3 {
		fmt.Println("usage: tinyaws exec <instance-id> -- <command...>")
		os.Exit(1)
	}

	instanceID := args[0]

	// find "--" separator
	cmdStart := -1
	for i, a := range args {
		if a == "--" {
			cmdStart = i + 1
			break
		}
	}
	if cmdStart < 0 || cmdStart >= len(args) {
		fmt.Println("usage: tinyaws exec <instance-id> -- <command...>")
		os.Exit(1)
	}

	command := strings.Join(args[cmdStart:], " ")
	jobID := submitJobCommand(command, instanceID)
	fmt.Printf("exec job %s on instance %s\n", jobID, instanceID)

	// poll until done
	for {
		job, err := fetchJob(jobID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if job.Status == "done" || job.Status == "failed" {
			if job.Stdout != "" {
				fmt.Print(job.Stdout)
			}
			if job.Stderr != "" {
				fmt.Fprintf(os.Stderr, "%s", job.Stderr)
			}
			if job.ExitCode != nil && *job.ExitCode != 0 {
				os.Exit(*job.ExitCode)
			}
			break
		}
		time.Sleep(time.Second)
	}
}
