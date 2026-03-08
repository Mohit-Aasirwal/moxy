package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"moxy/pkg/crdt"
	"moxy/pkg/engine"
	"moxy/pkg/identity"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	chatRoomFlag     string
	chatPortFlag     int
	chatIdentityFlag string
	chatPasswordFlag string
)

// Lipgloss Styles
var (
	headerStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	infoStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A0A0A0")).
		MarginLeft(2)

	peerStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#04B575")).
		Bold(true).
		MarginRight(1)

	meStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00BFFF")).
		Bold(true).
		MarginRight(1)

	msgStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E0E0E0"))

	containerStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#444444"))
)

type uiModel struct {
	engine    *engine.Engine
	messages  []crdt.Message
	textInput textinput.Model
	viewport  viewport.Model
	ready     bool
	width     int
	height    int
}

type incomingMsg crdt.Message

func initialModel(eng *engine.Engine) uiModel {
	ti := textinput.New()
	ti.Placeholder = "Type a message and press Enter..."
	ti.Focus()
	ti.CharLimit = 500

	// Preload messages from engine DB securely decoupled
	preload := eng.History()

	// Sort chronologically (oldest to newest)
	sort.Slice(preload, func(i, j int) bool {
		if preload[i].Timestamp == preload[j].Timestamp {
			return preload[i].Clock < preload[j].Clock
		}
		return preload[i].Timestamp < preload[j].Timestamp
	})

	return uiModel{
		engine:    eng,
		textInput: ti,
		messages:  preload,
	}
}

func (m uiModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.waitForMessage())
}

func (m uiModel) waitForMessage() tea.Cmd {
	return func() tea.Msg {
		msg := <-m.engine.Outbox
		return incomingMsg(msg)
	}
}

func (m uiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textInput, tiCmd = m.textInput.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		headerHeight := 3
		footerHeight := 3
		vpHeight := m.height - headerHeight - footerHeight - 2

		if !m.ready {
			m.viewport = viewport.New(m.width-4, vpHeight)
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
			m.ready = true
		} else {
			m.viewport.Width = m.width - 4
			m.viewport.Height = vpHeight
			m.viewport.SetContent(m.renderMessages())
		}

	case incomingMsg:
		m.messages = append(m.messages, crdt.Message(msg))
		if m.ready {
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
		}
		return m, m.waitForMessage()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			txt := m.textInput.Value()
			if txt != "" {
				m.engine.Send(txt)
				m.textInput.Reset()
			}
		case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown:
			m.viewport, vpCmd = m.viewport.Update(msg)
			return m, vpCmd
		}
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m uiModel) renderMessages() string {
	var lines []string
	for _, msg := range m.messages {
		shortID := msg.Sender[:8]
		var line string
		if msg.Sender == m.engine.PeerID {
			line = meStyle.Render("You ["+shortID+"]:") + msgStyle.Render(msg.Content)
		} else {
			line = peerStyle.Render(shortID+":") + msgStyle.Render(msg.Content)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (m uiModel) View() string {
	if !m.ready {
		return "\n  Initializing P2P CRDT sync...\n"
	}

	// Header
	lockStatus := "🔓 Public"
	if m.engine.Password != "" {
		lockStatus = "🔒 Encrypted"
	}
	header := headerStyle.Render(fmt.Sprintf(" 🌐 Room: %s ", m.engine.Room))
	info := infoStyle.Render(fmt.Sprintf("%s | Nodes: %d | Local DB: %d msgs", lockStatus, len(m.engine.Node.Topic.ListPeers())+1, len(m.messages)))
	topRow := header + info + "\n"

	// Chat Box
	chatBox := containerStyle.
		Width(m.width - 2).
		Height(m.viewport.Height + 2).
		Render(m.viewport.View())

	// Input Box
	inputView := lipgloss.NewStyle().Padding(1, 2).Render(m.textInput.View())

	return fmt.Sprintf("\n%s\n%s\n%s", topRow, chatBox, inputView)
}

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Launch the interactive Moxy chat UI",
	Run: func(cmd *cobra.Command, args []string) {
		home, _ := os.UserHomeDir()
		path := chatIdentityFlag
		if path == "" {
			path = filepath.Join(home, ".moxy", "identity.json")
		}

		ident, priv, err := identity.LoadOrGenerate(path)
		if err != nil {
			fmt.Printf("Fatal: Could not load or generate identity: %v\n", err)
			os.Exit(1)
		}

		port := chatPortFlag
		if port == 0 {
			port, err = engine.GetFreePort()
			if err != nil {
				fmt.Printf("Fatal: Could not find free port: %v\n", err)
				os.Exit(1)
			}
		}

		dbPath := filepath.Join(home, ".moxy", "store_"+ident.PeerID)
		
		ctx := context.Background()
		eng, err := engine.NewEngine(ctx, priv, ident.PeerID, dbPath, chatRoomFlag, chatPasswordFlag, port)
		if err != nil {
			if strings.Contains(err.Error(), "Another process is using this Badger database") {
				fmt.Printf("\n❌ Fatal: Another Moxy instance is currently running using this identity!\n\n💡 If you are testing two Moxy instances on the *same laptop*, you must give the second terminal its own identity file so they don't fight over the same database:\n\n   ./moxy chat -r \"%s\" -i ~/.moxy/hacker.json\n\n", chatRoomFlag)
			} else {
				fmt.Printf("Fatal: Could not initialize engine: %v\n", err)
			}
			os.Exit(1)
		}
		defer eng.Close()

		p := tea.NewProgram(initialModel(eng), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Printf("Fatal: UI error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	chatCmd.Flags().StringVarP(&chatRoomFlag, "room", "r", "disaster", "Room name to join")
	chatCmd.Flags().IntVarP(&chatPortFlag, "port", "p", 0, "Listen port (0 = auto)")
	chatCmd.Flags().StringVarP(&chatIdentityFlag, "identity", "i", "", "Path to identity file (default: ~/.moxy/identity.json)")
	chatCmd.Flags().StringVar(&chatPasswordFlag, "password", "", "Symmetric room password for E2EE (optional)")
	rootCmd.AddCommand(chatCmd)
}
