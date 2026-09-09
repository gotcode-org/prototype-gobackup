package engine

import (
	"log"
	"time"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// GitOpsSync stages, commits, and pushes the configuration directory using pure Go
func GitOpsSync(configDir string, commitMessage string) error {
	log.Printf("🐙 GitOps: Syncing changes for %s", commitMessage)

	// 1. Open the repository
	r, err := git.PlainOpen(configDir)
	if err != nil {
		log.Printf("GitOps failed to open repo: %v", err)
		return err
	}

	w, err := r.Worktree()
	if err != nil {
		return err
	}

	// 2. git add .
	err = w.AddWithOptions(&git.AddOptions{All: true})
	if err != nil {
		log.Printf("GitOps add failed: %v", err)
		return err
	}

	// 3. git commit
	commit, err := w.Commit(commitMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "GoBackup Daemon",
			Email: "daemon@gobackup.local",
			When:  time.Now(),
		},
	})
	if err != nil {
		log.Printf("GitOps commit (safe failure if no changes): %v", err)
		return nil
	}

	obj, _ := r.CommitObject(commit)
	log.Printf("GitOps committed: %s", obj.Hash.String())

	// 4. Configure SSH Authentication
	// go-git requires explicit auth configuration for SSH
	var auth *ssh.PublicKeys
	home, _ := os.UserHomeDir()
	keyPaths := []string{
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "id_rsa"),
	}

	for _, keyPath := range keyPaths {
		if _, err := os.Stat(keyPath); err == nil {
			auth, _ = ssh.NewPublicKeysFromFile("git", keyPath, "")
			if auth != nil {
				break
			}
		}
	}

	// 5. git push (Explicitly targeting the remote 'main' branch)
	pushOpts := &git.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []gitconfig.RefSpec{"refs/heads/main:refs/heads/main"},
	}
	if auth != nil {
		pushOpts.Auth = auth
	}

	err = r.Push(pushOpts)
	if err != nil {
		log.Printf("GitOps push failed: %v", err)
		return err
	}

	log.Printf("🐙 GitOps: Successfully pushed configuration to remote!")
	return nil
}
