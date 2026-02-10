package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func stateCmd(args []string) {
	fs := flag.NewFlagSet("state", flag.ExitOnError)
	baseURL := fs.String("url", "http://127.0.0.1:8080", "server base url")
	_ = fs.Parse(args)

	u := strings.TrimRight(strings.TrimSpace(*baseURL), "/") + "/admin/v1/state"
	cl := &http.Client{Timeout: 5 * time.Second}
	resp, err := cl.Get(u)
	if err != nil {
		fmt.Fprintln(os.Stderr, "request:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	fmt.Println(string(b))
	if resp.StatusCode/100 != 2 {
		os.Exit(1)
	}
}

func snapshotCmd(args []string) {
	fs := flag.NewFlagSet("snapshot", flag.ExitOnError)
	baseURL := fs.String("url", "http://127.0.0.1:8080", "server base url")
	_ = fs.Parse(args)

	u := strings.TrimRight(strings.TrimSpace(*baseURL), "/") + "/admin/v1/snapshot"
	req, _ := http.NewRequest(http.MethodPost, u, nil)
	cl := &http.Client{Timeout: 10 * time.Second}
	resp, err := cl.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "request:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	fmt.Println(string(b))
	if resp.StatusCode/100 != 2 {
		os.Exit(1)
	}
}
