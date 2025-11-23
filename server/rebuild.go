package server

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
)

// RebuildToken is a simple token to protect the rebuild endpoint
// Not super secure, but prevents accidental triggers
const RebuildToken = "cardgame123"

// HandleRebuild handles the /rebuild endpoint
// It pulls latest code, rebuilds, and exits so systemd can restart
func HandleRebuild(w http.ResponseWriter, r *http.Request) {
	// Check token
	token := r.URL.Query().Get("token")
	if token != RebuildToken {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	log.Println("Rebuild requested - starting update process...")

	// Step 1: Git pull
	log.Println("Running git pull...")
	gitCmd := exec.Command("git", "pull", "origin", "main")
	gitCmd.Dir = "."
	gitOutput, err := gitCmd.CombinedOutput()
	if err != nil {
		errMsg := fmt.Sprintf("Git pull failed: %v\n%s", err, string(gitOutput))
		log.Println(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}
	log.Printf("Git pull output: %s", string(gitOutput))

	// Step 2: Go build
	log.Println("Running go build...")
	buildCmd := exec.Command("go", "build", "-o", "cardgame", "./cmd/server")
	buildCmd.Dir = "."
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		errMsg := fmt.Sprintf("Build failed: %v\n%s", err, string(buildOutput))
		log.Println(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}
	log.Printf("Build output: %s", string(buildOutput))

	// Step 3: Respond success before exiting
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Rebuild successful! Server restarting...")
	log.Println("Rebuild complete - exiting for systemd restart...")

	// Flush the response
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Exit gracefully - systemd will restart us
	go func() {
		os.Exit(0)
	}()
}
