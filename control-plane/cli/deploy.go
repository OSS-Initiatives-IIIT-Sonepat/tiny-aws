package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Handles: tinyaws deploy <dir> [--instance i-1] [--wait] [--service] [--port N]
func runDeploy(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: tinyaws deploy <dir> [--instance i-1] [--wait] [--service] [--port N]")
		os.Exit(1)
	}

	srcDir := args[0]
	instanceID := ""
	wait := false
	isService := false
	port := 0

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--instance":
			if i+1 >= len(args) {
				fmt.Println("--instance requires a value")
				os.Exit(1)
			}
			instanceID = args[i+1]
			i++
		case "--wait":
			wait = true
		case "--service":
			isService = true
		case "--port":
			if i+1 >= len(args) {
				fmt.Println("--port requires a value")
				os.Exit(1)
			}
			fmt.Sscanf(args[i+1], "%d", &port)
			i++
		}
	}

	zipPath, err := zipDirectory(srcDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "zip failed: %v\n", err)
		os.Exit(1)
	}
	defer os.Remove(zipPath)

	zipBytes, err := os.ReadFile(zipPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read zip: %v\n", err)
		os.Exit(1)
	}

	key := filepath.Base(srcDir) + ".zip"
	ensureBucket("deployments")
	uploadObject("deployments", key, zipBytes)

	storeURL := objectStoreURL()
	downloadURL := fmt.Sprintf("%s/buckets/deployments/objects/%s", storeURL, key)

	jobID := submitDeployJob(downloadURL, instanceID, isService, port)
	if isService {
		fmt.Printf("service deploy job %s started (port %d)\n", jobID, port)
	} else {
		fmt.Printf("deploy job %s started\n", jobID)
	}

	if wait {
		for {
			job, err := fetchJob(jobID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			if job.Status == "done" || job.Status == "failed" {
				printJob(job)
				break
			}
			time.Sleep(2 * time.Second)
		}
	}
}

func zipDirectory(src string) (string, error) {
	tmp, err := os.CreateTemp("", "tinyaws-*.zip")
	if err != nil {
		return "", err
	}
	tmp.Close()

	out, err := os.Create(tmp.Name())
	if err != nil {
		return "", err
	}
	defer out.Close()

	w := zip.NewWriter(out)
	defer w.Close()

	err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		rel = strings.ReplaceAll(rel, `\`, `/`)

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		hdr.Name = rel

		writer, err := w.CreateHeader(hdr)
		if err != nil {
			return err
		}
		_, err = io.Copy(writer, f)
		return err
	})
	if err != nil {
		return "", err
	}
	return tmp.Name(), nil
}

func ensureBucket(name string) {
	req, _ := http.NewRequest(http.MethodPut, objectStoreURL()+"/buckets/"+name, nil)
	resp, _ := httpDo(req)
	if resp != nil {
		resp.Body.Close()
	}
}

func uploadObject(bucket, key string, data []byte) {
	req, err := http.NewRequest(http.MethodPut, objectURL(bucket, key), bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "upload failed: %v\n", err)
		os.Exit(1)
	}

	resp, err := httpDo(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "upload failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "upload failed %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}
}

func submitJobCommand(command, instanceID string) string {
	payload := map[string]string{"command": command}
	if instanceID != "" {
		payload["instance_id"] = instanceID
	}
	b, _ := json.Marshal(payload)

	resp, err := httpPost(schedulerURL()+"/jobs", "application/json", bytes.NewReader(b))
	if err != nil {
		fmt.Fprintf(os.Stderr, "submit failed: %v\n", err)
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
	return job.ID
}

// submitDeployJob submits a deploy job with deploy_url; agent handles download/run.
func submitDeployJob(deployURL, instanceID string, isService bool, port int) string {
	payload := map[string]any{"deploy_url": deployURL, "command": ""}
	if instanceID != "" {
		payload["instance_id"] = instanceID
	}
	if isService {
		payload["job_type"] = "service"
		payload["port"] = port
	}
	b, _ := json.Marshal(payload)

	resp, err := httpPost(schedulerURL()+"/jobs", "application/json", bytes.NewReader(b))
	if err != nil {
		fmt.Fprintf(os.Stderr, "submit failed: %v\n", err)
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
	return job.ID
}
