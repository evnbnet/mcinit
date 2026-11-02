package ui

import (
	"fmt"
	"strings"
	"time"
)

// barWidth is the inner width of the progress bar in columns.
const barWidth = 24

// repaintEvery throttles redraws so a fast download does not spend its time
// writing escape codes.
const repaintEvery = 80 * time.Millisecond

// Progress renders a single-line download bar that repaints in place. It is an
// io.Writer, so it goes straight into an io.TeeReader around the response body.
//
// When stdout is not a terminal it prints one static line and then stays quiet,
// because repainting with \r turns into garbage in a log file.
type Progress struct {
	label  string
	total  int64
	done   int64
	frame  int
	last   time.Time
	silent bool
}

// NewProgress starts a bar. total may be 0 when the size is unknown, in which
// case the bar is replaced by a running byte count.
func NewProgress(label string, total int64) *Progress {
	p := &Progress{label: label, total: total, silent: !interactive}
	if p.silent {
		fmt.Printf("%s%s %s\n", indent, mutedStyle.Render(glyphs.spinner[0]), label)
		return p
	}
	p.paint()
	return p
}

// Write counts bytes and repaints. It never fails.
func (p *Progress) Write(b []byte) (int, error) {
	p.done += int64(len(b))
	if !p.silent && time.Since(p.last) >= repaintEvery {
		p.frame++
		p.paint()
	}
	return len(b), nil
}

// Finish erases the bar so the caller can print a final Ok line in its place.
func (p *Progress) Finish() {
	if p.silent {
		return
	}
	fmt.Print("\r\033[K")
}

func (p *Progress) paint() {
	p.last = time.Now()

	spin := accentStyle.Render(glyphs.spinner[p.frame%len(glyphs.spinner)])
	var tail string

	if p.total > 0 {
		ratio := float64(p.done) / float64(p.total)
		if ratio > 1 {
			ratio = 1
		}
		filled := int(ratio * barWidth)
		bar := accentStyle.Render(strings.Repeat(glyphs.barFull, filled)) +
			mutedStyle.Render(strings.Repeat(glyphs.barEmpty, barWidth-filled))
		tail = fmt.Sprintf("%s  %s", bar, mutedStyle.Render(fmt.Sprintf("%3.0f%%", ratio*100)))
	} else {
		tail = mutedStyle.Render(humanBytes(p.done))
	}

	fmt.Printf("\r\033[K%s%s %s  %s", indent, spin, p.label, tail)
}

// humanBytes formats a byte count for a one-line display.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	value, units := float64(n), []string{"KB", "MB", "GB", "TB"}
	for _, u := range units {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, u)
		}
	}
	return fmt.Sprintf("%.1f PB", value/unit)
}

// Spin runs work while showing a spinner on one line, then erases it. Used for
// the steps with no measurable progress, like probing for Java or waiting on
// winget.
func Spin(label string, work func() error) error {
	if !interactive {
		fmt.Printf("%s%s %s\n", indent, mutedStyle.Render(glyphs.spinner[0]), label)
		return work()
	}

	done := make(chan error, 1)
	go func() { done <- work() }()

	ticker := time.NewTicker(repaintEvery)
	defer ticker.Stop()

	for frame := 0; ; frame++ {
		select {
		case err := <-done:
			fmt.Print("\r\033[K")
			return err
		case <-ticker.C:
			fmt.Printf("\r\033[K%s%s %s", indent,
				accentStyle.Render(glyphs.spinner[frame%len(glyphs.spinner)]), label)
		}
	}
}
