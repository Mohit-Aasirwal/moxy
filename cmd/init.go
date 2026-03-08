package cmd

import (
	"fmt"
	"log"
	"moxy/pkg/identity"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initIdentityFlag string

var initCmd = &cobra.Command{
    Use:   "init",
    Short: "Initialize Moxy identity",
    Run: func(cmd *cobra.Command, args []string) {
        path := initIdentityFlag
        if path == "" {
             home, _ := os.UserHomeDir()
             path = filepath.Join(home, ".moxy", "identity.json")
        }

        if _, err := os.Stat(path); err == nil {
            fmt.Println("Identity already exists at", path)
            return
        }

        ident, _, err := identity.GenerateKeyPair()
        if err != nil {
            log.Fatalf("Failed to generate identity: %v", err)
        }

        if err := ident.Save(path); err != nil {
            log.Fatalf("Failed to save identity: %v", err)
        }

        fmt.Println("Generated new identity:")
        fmt.Println("PeerID:", ident.PeerID)
        fmt.Println("Saved to:", path)
    },
}

func init() {
    initCmd.Flags().StringVarP(&initIdentityFlag, "identity", "i", "", "Path to save identity file (default: ~/.moxy/identity.json)")
    rootCmd.AddCommand(initCmd)
}
