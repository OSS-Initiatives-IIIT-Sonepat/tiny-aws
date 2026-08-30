package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// Job JSON from scheduler.
type jobRecord struct {
	ID       string `json:"job_id"`
	NodeID   string `json:"node_id"`
	Command  string `json:"command"`
	Status   string `json:"status"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode *int   `json:"exit_code"`
}

// Handles: tinyaws job submit|status ...
func runJob(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: tinyaws job submit|status ...")
		os.Exit(1)
	}

	switch args[0] {
	case "submit":
		runJobSubmit(args[1:])
	case "status":
		runJobStatus(args[1:])
	default:
		fmt.Println("usage: tinyaws job submit|status ...")
		os.Exit(1)
	}
}

// POST /jobs with {"command":"..."} and optional instance_id.
func runJobSubmit(args []string) {
	if len(args) < 1 {
		fmt.Println(`usage: tinyaws job submit "echo hello" [--instance i-1]`)
		os.Exit(1)
	}

	command := args[0]
	instanceID := ""

	for i := 1; i < len(args); i++ {
		if args[i] == "--instance" {
			if i+1 >= len(args) {
				fmt.Println("--instance requires a value")
				os.Exit(1)
			}
			instanceID = args[i+1]
			i++
		}
	}

	payload := map[string]string{"command": command}
	if instanceID != "" {
		payload["instance_id"] = instanceID
	}
	b, _ := json.Marshal(payload)

	resp, err := httpPost(schedulerURL()+"/jobs", "application/json", bytes.NewReader(b))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		fmt.Fprintf(os.Stderr, "submit failed %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	var job jobRecord
	if err := json.Unmarshal(body, &job); err != nil {
		fmt.Fprintf(os.Stderr, "decode: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("submitted %s on node %s (status=%s)\n", job.ID, job.NodeID, job.Status)
}

// GET /jobs/{id} — optional --wait polls until done/failed.
func runJobStatus(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: tinyaws job status <job-id> [--wait]")
		os.Exit(1)
	}

	jobID := args[0]
	wait := len(args) >= 2 && args[1] == "--wait"

	for {
		job, err := fetchJob(jobID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		printJob(job)

		if !wait || job.Status == "done" || job.Status == "failed" {
			break
		}

		time.Sleep(2 * time.Second)
	}
}

// GET /jobs/{id} once.
func fetchJob(jobID string) (*jobRecord, error) {
	url := schedulerURL() + "/jobs/" + jobID
	resp, err := httpGet(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("scheduler returned %d: %s", resp.StatusCode, string(body))
	}

	var job jobRecord
	if err := json.Unmarshal(body, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// Pretty-print job fields.
func printJob(job *jobRecord) {
	fmt.Printf("job_id=%s node_id=%s status=%s command=%q\n",
		job.ID, job.NodeID, job.Status, job.Command)
	if job.ExitCode != nil {
		fmt.Printf("exit_code=%d\n", *job.ExitCode)
	}
	if job.Stdout != "" {
		fmt.Printf("stdout:\n%s", job.Stdout)
	}
	if job.Stderr != "" {
		fmt.Printf("stderr:\n%s", job.Stderr)
	}
}
