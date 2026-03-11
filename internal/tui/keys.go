package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Start    key.Binding
	Stop     key.Binding
	Restart  key.Binding
	Quit     key.Binding
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	IncVDev  key.Binding
	DecVDev  key.Binding
	IncDepth key.Binding
	DecDepth key.Binding
	IncSleep key.Binding
	DecSleep key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Start:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "start")),
		Stop:     key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "stop")),
		Restart:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restart")),
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("up/k", "scroll up")),
		Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("down/j", "scroll down")),
		PageUp:   key.NewBinding(key.WithKeys("pgup", "b"), key.WithHelp("pgup/b", "page up")),
		PageDown: key.NewBinding(key.WithKeys("pgdown", "f"), key.WithHelp("pgdn/f", "page down")),
		IncVDev:  key.NewBinding(key.WithKeys(">", "."), key.WithHelp(">", "vdev +")),
		DecVDev:  key.NewBinding(key.WithKeys("<", ","), key.WithHelp("<", "vdev -")),
		IncDepth: key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "depth +")),
		DecDepth: key.NewBinding(key.WithKeys("["), key.WithHelp("[", "depth -")),
		IncSleep: key.NewBinding(key.WithKeys("+", "="), key.WithHelp("+", "sleep +")),
		DecSleep: key.NewBinding(key.WithKeys("-"), key.WithHelp("-", "sleep -")),
	}
}
