package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/jroimartin/gocui"
)

func main() {
	g, err := gocui.NewGui(gocui.Output256)
	if err != nil {
		log.Panicln(err)
	}
	defer g.Close()

	grid := []struct {
		Name   string
		Width  int
		Height int
		XPos   int
		YPos   int
	}{
		{"Sidebar", 3, 10, 0, 0}, // Left sidebar (3 columns, 10 rows)
		{"command", 9, 12, 3, 0}, // Main content (9 columns, 10 rows)
		{"help", 3, 2, 0, 10},    // Full-width header (12 columns, 2 rows)
	}

	started := false

	targetsMap := readMakefile() // Now returns map[string]string

	g.SetManagerFunc(func(gui *gocui.Gui) error {
		err := GridLayout(g, grid)
		if err != nil {
			return err
		}
		if started == false {
			started = true
			err = initViews(g, targetsMap)
			if err != nil {
				return err
			}
		} else {

			err := updateViews(g, targetsMap)
			if err != nil {
				return err
			}
		}

		return nil
	})

	//g.SetManagerFunc(layout)

	if err := keybindings(g); err != nil {
		log.Panicln(err)
	}

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}
}

func updateViews(g *gocui.Gui, targetsMap map[string]string) error {

	v, err := g.View("Sidebar")
	if err != nil {
		return err
	}
	_, cy := v.Cursor()
	line, err := v.Line(cy)

	v2, err := g.View("help")
	if err != nil {
		return err
	}
	v2.Clear()
	doc := ""
	if line != "" {
		doc = targetsMap[line]
	}
	fmt.Fprintf(v2, "%s", doc)
	if doc != "" {
		v2.Wrap = true
	}
	return nil
}

func initViews(g *gocui.Gui, targetsMap map[string]string) error {
	v, err := g.View("Sidebar")
	if err != nil {
		return err
	}
	v.Title = "Makefile Targets"
	v.SelBgColor = gocui.ColorBlue
	v.SelFgColor = gocui.ColorBlack
	v.Highlight = true
	for target, _ := range targetsMap {
		_, err := fmt.Fprintf(v, "%s\n", target)
		if err != nil {
			return err
		}
	}
	_, err = g.SetCurrentView("Sidebar")
	if err != nil {
		return err
	}
	v2, err := g.View("command")
	if err != nil {
		return err
	}
	v2.Title = "Command Output"
	v2.Autoscroll = true
	return nil
}

// GridLayout takes the gocui.Gui object and a grid configuration with view names,
// and divides the screen into a dynamic grid layout based on the given configuration.
func GridLayout(g *gocui.Gui, grid []struct {
	Name   string // Name of the view
	Width  int    // Width in grid units (1-12)
	Height int    // Height in grid units (1-12)
	XPos   int    // X position in grid units
	YPos   int    // Y position in grid units
}) error {
	maxX, maxY := g.Size()

	// Calculate the unit width and height as floats
	unitX := float64(maxX) / 12.0
	unitY := float64(maxY) / 12.0

	for _, section := range grid {
		// Calculate the exact width, height, X position, and Y position using floats
		width := float64(section.Width) * unitX
		height := float64(section.Height) * unitY
		xPos := float64(section.XPos) * unitX
		yPos := float64(section.YPos) * unitY

		// Convert to integers, subtract 1 from width and height to prevent boundary overflow
		x0, y0 := int(xPos), int(yPos)
		x1, y1 := int(xPos+width)-1, int(yPos+height)-1
		if x1 <= 0 && y1 <= 0 {
			x1 = 1
			y1 = 1
		}

		// Create the view using the given name
		if v, err := g.SetView(section.Name, x0, y0, x1, y1); err != nil {
			if !errors.Is(err, gocui.ErrUnknownView) {
				return err
			}

			v.Title = section.Name // Set the title to the view's name
		}
	}

	return nil
}

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	v, err := g.SetView("makefile", 0, 0, maxX/2+1, int(math.Max(float64(maxY+1), 0)))
	if err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Title = "Makefile Targets"
		v.SelBgColor = gocui.ColorBlue
		v.SelFgColor = gocui.ColorBlack
		v.Highlight = true
		targetsMap := readMakefile() // Now returns map[string]string
		for target, doc := range targetsMap {
			_, err := fmt.Fprintf(v, "%s: %s\n", target, doc)
			if err != nil {
				return err
			}
		}
	}
	if v, err := g.SetView("command", maxX/2, 0, maxX+1, maxY+1); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Title = "Command Output"
	}
	_, err = g.SetCurrentView("makefile")
	if err != nil {
		return err
	}
	return nil
}

func keybindings(g *gocui.Gui) error {
	if err := g.SetKeybinding("", gocui.KeyArrowDown, gocui.ModNone, cursorDown); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyArrowUp, gocui.ModNone, cursorUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyEnter, gocui.ModNone, executeCommand); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}
	return nil
}

func cursorDown(g *gocui.Gui, v *gocui.View) error {
	v.MoveCursor(0, 1, false)
	return nil
}

func cursorUp(g *gocui.Gui, v *gocui.View) error {
	v.MoveCursor(0, -1, false)
	return nil
}

func executeCommand(g *gocui.Gui, v *gocui.View) error {
	_, cy := v.Cursor()
	line, err := v.Line(cy)
	if err != nil {
		return err
	}

	g.Update(func(g *gocui.Gui) error {
		cmdView, err := g.View("command")
		if err != nil {
			return err
		}
		cmdView.Clear()

		// Create the command
		cmd := exec.Command("make", line)

		// Get stdout pipe
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}

		// Start the command
		if err := cmd.Start(); err != nil {
			return err
		}

		// Create a goroutine to stream output
		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				outputLine := scanner.Text()
				g.Update(func(g *gocui.Gui) error {
					fmt.Fprintln(cmdView, outputLine)
					return nil
				})
			}
			if err := scanner.Err(); err != nil {
				g.Update(func(g *gocui.Gui) error {
					fmt.Fprintln(cmdView, "Error reading command output:", err)
					return nil
				})
			}
		}()

		return nil
	})

	return nil
}
func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func readMakefile() map[string]string {
	file, err := os.Open("Makefile")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	targetsMap := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, ":") &&
			!strings.HasPrefix(line, "\t") &&
			!strings.HasPrefix(line, ".") &&
			!strings.Contains(line, "PHONY") &&
			regexp.MustCompile(`^[a-zA-Z0-9_-]+:`).MatchString(line) {
			parts := strings.SplitN(line, ":", 2)
			target := parts[0]
			doc := strings.TrimSpace(parts[1]) // Assuming the documentation follows the colon
			targetsMap[target] = doc
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return targetsMap
}
