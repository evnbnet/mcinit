package ui

import (
	"fmt"
	"strings"
	"time"
)

const barWidth = 24

const repaintEvery = 80 * time.Millisecond

type Progress struct {
	label  string
	total  int64
	done   int64
	frame  int
	last   time.Time
	silent bool
}

func NewProgress(label string, total int64) *Progress {
	p := &Progress{label: label, total: total, silent: !interactive}
	if p.silent {
		fmt.Printf("%s%s %s\n", indent, mutedStyle.Render(glyphs.spinner[0]), label)
		return p
	}
	p.paint()
	return p
}

func (p *Progress) Write(b []byte) (int, error) {
	p.done += int64(len(b))
	if !p.silent && time.Since(p.last) >= repaintEvery {
		p.frame++
		p.paint()
	}
	return len(b), nil
}

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
