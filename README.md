# seventys_tui

A retro 70s sci-fi terminal TUI built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Beep](https://github.com/faiface/beep). Keys chirp as you type; the computer "prints" responses with a teletype effect and matching beeps.

## Requirements

- Go 1.22+
- Audio output (speakers or headphones)

## Run

```bash
go run .
```

## Controls

- **Type** — adds characters with an 800 Hz typewriter chirp
- **Enter** — triggers a simulated "PROCESSING REQUEST..." response sequence
- **Esc** or **Ctrl+C** — exit

The startup banner streams onto the screen character by character with 1200 Hz computer chirps (spaces and newlines are silent).
