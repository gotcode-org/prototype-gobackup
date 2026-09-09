package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type BackupUI interface {
	Log(format string, args ...interface{})
	Summary(format string, args ...interface{})
	SetStatus(status string, spinning bool)
	Start() error
	Stop()
}

// --- Raw UI (For --cron) ---
type RawUI struct{
	done chan bool
}
func NewRawUI() *RawUI { return &RawUI{done: make(chan bool)} }

func (u *RawUI) Log(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}
func (u *RawUI) Summary(format string, args ...interface{}) { fmt.Printf(format+"\n", args...) }
func (u *RawUI) SetStatus(status string, spinning bool) {
	fmt.Printf("\n>>> %s\n", status)
}
func (u *RawUI) Start() error { <-u.done; return nil }
func (u *RawUI) Stop()        { close(u.done) }

// --- Terminal UI (For Humans) ---
type TviewUI struct {
	app          *tview.Application
	statusView   *tview.TextView
	logView      *tview.TextView
	statusText   string
	isSpinning   bool
	stopSpinner  chan bool
	summaryLines []string
}

func NewTviewUI() *TviewUI {
	app := tview.NewApplication()

	statusView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	statusView.SetBorder(true).SetTitle(" Status ")

	logView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetMaxLines(1000)
	logView.SetBorder(true).SetTitle(" Real-Time Logs ")
	logView.SetChangedFunc(func() { app.Draw() })

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(statusView, 16, 1, false).
		AddItem(logView, 0, 3, false)

	app.SetRoot(flex, true)

	ui := &TviewUI{
		app:         app,
		statusView:  statusView,
		logView:     logView,
		stopSpinner: make(chan bool),
	}
	ui.startSpinnerLoop()
	return ui
}

func (u *TviewUI) Log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	safeMsg := tview.Escape(msg)
	
	u.app.QueueUpdateDraw(func() {
		fmt.Fprintf(u.logView, "%s\n", safeMsg)
		u.logView.ScrollToEnd()
	})
}

func (u *TviewUI) Summary(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	u.app.QueueUpdateDraw(func() {
		u.summaryLines = append(u.summaryLines, msg)
		if len(u.summaryLines) > 10 {
			u.summaryLines = u.summaryLines[len(u.summaryLines)-10:]
		}
	})
}

func (u *TviewUI) SetStatus(status string, spinning bool) {
	u.statusText = status
	u.isSpinning = spinning
}

func (u *TviewUI) startSpinnerLoop() {
	spinChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	go func() {
		for {
			select {
			case <-u.stopSpinner:
				return
			default:
				if u.statusText != "" {
					var text string
					
					history := strings.Join(u.summaryLines, "\n")
					if history != "" {
						history = "\n\n" + history
					}
					if u.isSpinning {
						text = fmt.Sprintf("[yellow::b]%s %s[-::-]%s", spinChars[i], u.statusText, history)
						i = (i + 1) % len(spinChars)
					} else {
						text = fmt.Sprintf("[green::b]%s[-::-]%s", u.statusText, history)
					}
					u.app.QueueUpdateDraw(func() {
						u.statusView.SetText(text)
					})
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

func (u *TviewUI) Start() error { return u.app.Run() }

func (u *TviewUI) Stop() {
	u.app.QueueUpdateDraw(func() {
		u.statusView.SetText(u.statusView.GetText(false) + "\n\n[white::b]Backup finished! Press 'q', 'Enter', or 'Esc' to exit...[-::-]")
	})
	u.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' || event.Key() == tcell.KeyEnter || event.Key() == tcell.KeyEscape {
			u.app.Stop()
		}
		return event
	})
}
