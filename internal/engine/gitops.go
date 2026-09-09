package engine

import (
	"log"
	"os/exec"
)

// GitOpsSync stages, commits, and pushes the configuration directory to the remote SourceVault server
func GitOpsSync(configDir string, commitMessage string) error {
	log.Printf("🐙 GitOps: Syncing changes for %s", commitMessage)

	// git add .
	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = configDir
	if err := addCmd.Run(); err != nil {
		log.Printf("GitOps add failed: %v", err)
		return err
	}

	// git commit -m "..."
	commitCmd := exec.Command("git", "commit", "-m", commitMessage)
	commitCmd.Dir = configDir
	if err := commitCmd.Run(); err != nil {
		// Might fail if there are no changes, which is fine
		log.Printf("GitOps commit (safe failure if no changes): %v", err)
		return nil 
	}

	// git push
	pushCmd := exec.Command("git", "push")
	pushCmd.Dir = configDir
	if err := pushCmd.Run(); err != nil {
		log.Printf("GitOps push failed: %v", err)
		return err
	}

	log.Printf("🐙 GitOps: Successfully pushed configuration to SourceVault")
	return nil
}
