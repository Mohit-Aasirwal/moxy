package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "moxy",
	Short: "Moxy Interactive CLI",
	Long:  `A resilient, delay-tolerant CLI messaging system over LAN.`,
	Run: func(cmd *cobra.Command, args []string) {
		interactiveWizard()
	},
}

func interactiveWizard() {
	bannerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(1, 4).
		MarginBottom(1)

	fmt.Println()
	fmt.Println(bannerStyle.Render("Welcome to Moxy! 🚀"))
	fmt.Println("Moxy is a zero-config, delay-tolerant, local P2P messaging tool.")
	fmt.Println("No servers. No tracking. Completely decentralized.")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter a Room Name to join (leave blank for 'global'): ")
	room, _ := reader.ReadString('\n')
	room = strings.TrimSpace(room)
	if room == "" {
		room = "global"
	}

	fmt.Print("Enter a Secure Password for End-to-End Encryption (leave blank for public room): ")
	pass, _ := reader.ReadString('\n')
	pass = strings.TrimSpace(pass)

	fmt.Println("\nAwesome! Entering the Matrix... 🟢")
	
	// Inject wizard selections into the underlying UI Chat Command logic
	chatRoomFlag = room
	chatPasswordFlag = pass
	chatPortFlag = 0
	chatIdentityFlag = ""

	chatCmd.Run(chatCmd, []string{})
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
