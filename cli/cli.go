package cli

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/term"
)

var up byte = 65
var down byte = 66
var escape byte = 27
var enter byte = 13
var keys = map[byte]bool{
	up:   true,
	down: true,
}

type Menu struct {
	Prompt    string
	CursorPos int
	MenuItems []*MenuItem
}

type MenuItem struct {
	Text    string
	ID      string
	SubMenu *Menu
}

func NewMenu(prompt string) *Menu {
	return &Menu{
		Prompt:    prompt,
		MenuItems: make([]*MenuItem, 0),
	}
}

func (m *Menu) AddItem(option string, id string) *Menu {
	menuItem := &MenuItem{
		Text: option,
		ID:   id,
	}

	m.MenuItems = append(m.MenuItems, menuItem)
	return m
}

func (m *Menu) renderMenuItems(redraw bool) {
	if redraw {
		fmt.Printf("\033[%dA", len(m.MenuItems)-1)
	}

	for index, menuItem := range m.MenuItems {
		var newline = "\n"
		if index == len(m.MenuItems)-1 {
			newline = ""
		}

		menuItemText := menuItem.Text
		cursor := "  "
		if index == m.CursorPos {
			cursor = "\033[33m> \033[0m"                         // Yellow cursor
			menuItemText = "\033[33m" + menuItemText + "\033[0m" // Yellow text
		}

		fmt.Printf("\r%s %s%s", cursor, menuItemText, newline)
	}
}

func (m *Menu) Display() string {
	defer func() {
		fmt.Printf("\033[?25h") // Show cursor
	}()

	fmt.Printf("%s\n", "\033[36m"+"\033[1m"+m.Prompt+":"+"\033[0m") // Cyan and bold prompt

	m.renderMenuItems(false)

	fmt.Printf("\033[?25l") // Hide cursor

	for {
		keyCode := getInput()
		if keyCode == escape {
			return ""
		} else if keyCode == enter {
			menuItem := m.MenuItems[m.CursorPos]
			fmt.Println("\r")
			return menuItem.ID
		} else if keyCode == up {
			m.CursorPos = (m.CursorPos + len(m.MenuItems) - 1) % len(m.MenuItems)
			m.renderMenuItems(true)
		} else if keyCode == down {
			m.CursorPos = (m.CursorPos + 1) % len(m.MenuItems)
			m.renderMenuItems(true)
		}
	}
}

// getInput will read raw input from the terminal
// It returns the raw ASCII value inputted
func getInput() byte {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		log.Fatal(err)
	}
	defer term.Restore(fd, oldState)

	var read int
	readBytes := make([]byte, 3)
	read, err = os.Stdin.Read(readBytes)
	if err != nil {
		log.Fatal(err)
	}

	if read == 3 {
		if _, ok := keys[readBytes[2]]; ok {
			return readBytes[2]
		}
	} else {
		return readBytes[0]
	}

	return 0
}
