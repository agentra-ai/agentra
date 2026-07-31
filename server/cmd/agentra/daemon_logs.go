package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"
)

const maxDaemonLogLineBytes = 10 << 20

func streamDaemonLog(path string, lines int, follow bool, output io.Writer) error {
	if lines < 0 {
		return fmt.Errorf("--lines must be zero or greater")
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open daemon log: %w", err)
	}
	defer func() { _ = file.Close() }()

	if lines > 0 {
		if err := writeLastLogLines(file, lines, output); err != nil {
			return err
		}
	} else if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("seek daemon log: %w", err)
	}
	if !follow {
		return nil
	}

	offset, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("resolve daemon log offset: %w", err)
	}
	openedInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat daemon log: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), daemonSignals()...)
	defer stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			currentInfo, err := os.Stat(path)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return fmt.Errorf("stat daemon log: %w", err)
			}
			if currentInfo.Size() < offset || !os.SameFile(openedInfo, currentInfo) {
				file.Close()
				file, err = os.Open(path)
				if err != nil {
					return fmt.Errorf("reopen daemon log: %w", err)
				}
				openedInfo = currentInfo
				offset = 0
			}
			if currentInfo.Size() <= offset {
				continue
			}
			if _, err := file.Seek(offset, io.SeekStart); err != nil {
				return fmt.Errorf("seek daemon log: %w", err)
			}
			written, err := io.CopyN(output, file, currentInfo.Size()-offset)
			offset += written
			if err != nil {
				return fmt.Errorf("read daemon log: %w", err)
			}
		}
	}
}

func writeLastLogLines(file *os.File, lines int, output io.Writer) error {
	ring := make([]string, lines)
	count := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), maxDaemonLogLineBytes)
	for scanner.Scan() {
		ring[count%lines] = scanner.Text()
		count++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read daemon log: %w", err)
	}

	start := 0
	length := count
	if count > lines {
		start = count % lines
		length = lines
	}
	for i := 0; i < length; i++ {
		if _, err := fmt.Fprintln(output, ring[(start+i)%lines]); err != nil {
			return fmt.Errorf("write daemon log: %w", err)
		}
	}
	return nil
}
