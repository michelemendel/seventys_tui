package main

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
)

var (
	frameStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("2"))

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("2")).
			Align(lipgloss.Center)

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Align(lipgloss.Center)
)

const (
	sampleRate     = beep.SampleRate(44100)
	teletypeNormal = 30 * time.Millisecond
	teletypeFast   = 8 * time.Millisecond
	beepTyping      = 800.0
	beepTeletype    = 1050.0
	teletypeToneDur = 16 * time.Millisecond
)

func playTone(freq float64, duration time.Duration) {
	sr := float64(sampleRate)
	length := int(float64(duration) * sr / float64(time.Second))

	streamer := beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		for i := range samples {
			if length <= 0 {
				return i, false
			}
			v := math.Sin(2.0 * math.Pi * freq * float64(i) / sr)
			samples[i][0] = v * 0.1
			samples[i][1] = v * 0.1
			length--
		}
		return len(samples), true
	})
	speaker.Play(streamer)
}

// playTeletype is a soft sine blip with a smooth envelope — retro terminal, not a sharp tick.
func playTeletype(freq float64) {
	sr := float64(sampleRate)
	total := int(float64(teletypeToneDur) * sr / float64(time.Second))
	remaining := total

	streamer := beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		for i := range samples {
			if remaining <= 0 {
				return i, false
			}
			idx := total - remaining
			t := float64(idx) / sr
			progress := float64(idx) / float64(total-1)
			env := 0.5 * (1 - math.Cos(2*math.Pi*progress))

			phase := 2.0 * math.Pi * freq * t
			v := (math.Sin(phase) + math.Sin(phase*2)*0.12) * env * 0.09

			samples[i][0] = v
			samples[i][1] = v
			remaining--
		}
		return len(samples), true
	})
	speaker.Play(streamer)
}

type tickMsg time.Time

type model struct {
	input       string
	displayText string
	fullTarget  string
	printIndex  int
	printDelay  time.Duration
	audioChan   chan float64
	width       int
	height      int
}

func initialModel(audioChan chan float64) model {
	return model{
		fullTarget: "INITIALIZING MAINFRAME... READY.\nTYPE HELP FOR AVAILABLE COMMANDS.\n\n> ",
		printDelay: teletypeNormal,
		audioChan:  audioChan,
	}
}

func (m model) startTeletype(suffix string, delay time.Duration) (model, tea.Cmd) {
	m.fullTarget = m.displayText + suffix
	m.printIndex = len(m.displayText)
	m.printDelay = delay
	return m, tea.Tick(delay, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) canEditInput() bool {
	return m.printIndex >= len(m.fullTarget)
}

func (m model) withUserInput(s string) model {
	s = strings.ToUpper(s)
	m.input += s
	m.displayText += s
	m.audioChan <- beepTyping
	return m
}

func (m model) withBackspace() model {
	if !m.canEditInput() || m.input == "" {
		return m
	}
	m.input = m.input[:len(m.input)-1]
	m.displayText = m.displayText[:len(m.displayText)-1]
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(time.Millisecond*50, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}),
		tea.WindowSize(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case commandResultMsg:
		if msg.clear {
			m.displayText = "> "
			m.fullTarget = ""
			m.printIndex = 0
			return m, nil
		}
		delay := teletypeNormal
		if msg.fastPrint {
			delay = teletypeFast
		}
		if msg.response == "" {
			return m.startTeletype("\n> ", delay)
		}
		return m.startTeletype("\n"+msg.response+"\n\n> ", delay)

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			command := strings.TrimSpace(m.input)
			m.input = ""
			m.displayText += "\n"

			if strings.ToUpper(command) == "EXIT" {
				return m, tea.Quit
			}
			if command == "" {
				return m.startTeletype("> ", teletypeNormal)
			}
			return m, runCommand(command)
		case tea.KeyBackspace, tea.KeyCtrlH:
			return m.withBackspace(), nil
		case tea.KeySpace:
			return m.withUserInput(" "), nil
		case tea.KeyRunes:
			return m.withUserInput(string(msg.Runes)), nil
		}

	case tickMsg:
		if m.printIndex < len(m.fullTarget) {
			m.displayText += string(m.fullTarget[m.printIndex])
			m.printIndex++

			if m.fullTarget[m.printIndex-1] != ' ' && m.fullTarget[m.printIndex-1] != '\n' {
				m.audioChan <- beepTeletype
			}

			return m, tea.Tick(m.printDelay, func(t time.Time) tea.Msg {
				return tickMsg(t)
			})
		}
	}

	return m, nil
}

func visibleContent(text string, width, height int) string {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	lines := strings.Split(text, "\n")
	var wrapped []string
	for _, line := range lines {
		for len(line) > width {
			wrapped = append(wrapped, line[:width])
			line = line[width:]
		}
		wrapped = append(wrapped, line)
	}

	if len(wrapped) > height {
		wrapped = wrapped[len(wrapped)-height:]
	}
	for len(wrapped) < height {
		wrapped = append(wrapped, "")
	}

	return strings.Join(wrapped, "\n")
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	const header = "SEVENTYS MAINFRAME v1.0"
	const footer = "TYPE HELP FOR COMMANDS | EXIT TO QUIT"

	// Lipgloss borders add 2 cells; Width/Height apply to the inner content only.
	frameW := m.width - 2
	frameH := m.height - 2
	innerW := frameW
	innerH := frameH - 2 // header + footer
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}

	content := lipgloss.NewStyle().
		Width(innerW).
		Height(innerH).
		Render(visibleContent(m.displayText, innerW, innerH))

	inner := lipgloss.JoinVertical(
		lipgloss.Left,
		headerStyle.Width(innerW).Render(header),
		content,
		footerStyle.Width(innerW).Render(footer),
	)

	frame := frameStyle.
		Width(frameW).
		Height(frameH).
		Render(inner)

	return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, frame)
}

func main() {
	err := speaker.Init(sampleRate, sampleRate.N(time.Millisecond*10))
	if err != nil {
		fmt.Printf("Error initializing audio: %v\n", err)
		os.Exit(1)
	}

	audioChan := make(chan float64, 100)

	go func() {
		for freq := range audioChan {
			if freq == beepTeletype {
				playTeletype(freq)
				time.Sleep(time.Millisecond * 18)
			} else {
				playTone(freq, time.Millisecond*20)
				time.Sleep(time.Millisecond * 25)
			}
		}
	}()

	p := tea.NewProgram(initialModel(audioChan), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
