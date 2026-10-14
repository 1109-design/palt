package transfer

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

var (
	ErrRejected = errors.New("peer rejected the file transfer")

	// ErrTextUnsupported means the peer closed the connection without a
	// verdict, which is how PALT builds older than text support react.
	ErrTextUnsupported = errors.New("peer does not support text messages — update PALT on that device")
)

// SendFiles connects to a peer network endpoint, handshakes the batch metadata,
// and streams the chosen files sequentially from local disk.
// OnProgress tracks chunks traversing the network.
func SendFiles(dial Dialer, filePaths []string, transferID string, senderName string, onProgress func(written int64, total int64, sentItems int, totalItems int, currentFile string)) error {
	var totalSize int64 = 0
	var metaFiles []FileMeta

	// 1. Gather local file info
	for _, path := range filePaths {
		stat, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("failed to stat file %s: %w", path, err)
		}
		if stat.IsDir() {
			continue // Skip directories for simplicity right now
		}

		size := stat.Size()
		totalSize += size
		metaFiles = append(metaFiles, FileMeta{
			Name: filepath.Base(path),
			Size: size,
		})
	}

	if len(metaFiles) == 0 {
		return errors.New("no valid files selected to send")
	}

	// 2. Connect (direct TCP or an encrypted relay pipe)
	conn, err := dial()
	if err != nil {
		return fmt.Errorf("failed to reach peer: %w", err)
	}
	defer conn.Close()
	addr := conn.RemoteAddr().String()

	// 3. Construct and write Handshake
	meta := &Metadata{
		Action:     ActionOffer,
		TransferID: transferID,
		Files:      metaFiles,
		TotalSize:  totalSize,
		SenderName: senderName,
	}

	if err := WriteOffer(conn, meta); err != nil {
		return fmt.Errorf("failed to send handshake: %w", err)
	}

	// 4. Wait for Peer Verdict
	log.Printf("[TransferClient] Handshake sent! Waiting for %s verdict...", addr)
	verdictBuf := make([]byte, 1)
	if _, err := io.ReadFull(conn, verdictBuf); err != nil {
		return fmt.Errorf("dropped connection right before reading verdict: %w", err)
	}

	if verdictBuf[0] == ResponseReject {
		return ErrRejected
	} else if verdictBuf[0] != ResponseAccept {
		return fmt.Errorf("unexpected verdict byte from peer: 0x%x", verdictBuf[0])
	}

	// 5. Accepted! Stream files sequentially over same open socket.
	log.Printf("[TransferClient] Accepted! Sending %d files (%d bytes)...", len(metaFiles), totalSize)

	var totalRead int64 = 0
	totalFilesCount := len(filePaths)

	for i, path := range filePaths {
		file, err := os.Open(path)
		if err != nil {
			log.Printf("[TransferClient] Failed to open local file %s mid-transfer: %v", path, err)
			continue
		}

		stat, _ := file.Stat()
		if stat.IsDir() {
			file.Close()
			continue
		}

		tracker := &TrackingReader{
			In:            file,
			Total:         totalSize,
			ReadAmt:       totalRead,
			BroadcastStep: totalSize / 100, // roughly 1% updates
			OnProgress: func(read int64, total int64) {
				if onProgress != nil {
					onProgress(read, total, i+1, totalFilesCount, filepath.Base(path))
				}
			},
		}

		if tracker.BroadcastStep == 0 {
			tracker.BroadcastStep = 1024 * 1024 // Fallback 1MB
		}

		// 1MB buffer drastically reduces kernel syscall frequency
		buf := make([]byte, 1024*1024)
		sent, err := io.CopyBuffer(conn, tracker, buf)
		file.Close()
		totalRead += sent

		if err != nil {
			return fmt.Errorf("network interrupted mid-transfer. Wrote %d/%d bytes: %w", totalRead, totalSize, err)
		}
	}

	log.Printf("[TransferClient] Success! Wrote all %d bytes.", totalRead)
	return nil
}

// SendText delivers a single text message. The message rides inside the
// handshake, so the exchange is one frame out and one verdict byte back.
func SendText(dial Dialer, transferID string, senderName string, text string) error {
	if len(text) == 0 {
		return errors.New("message is empty")
	}
	if len(text) > MaxTextBytes {
		return fmt.Errorf("message is too long (max %d KB)", MaxTextBytes/1024)
	}

	conn, err := dial()
	if err != nil {
		return fmt.Errorf("failed to reach peer: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	meta := &Metadata{
		Action:     ActionText,
		TransferID: transferID,
		SenderName: senderName,
		Text:       text,
	}
	if err := WriteOffer(conn, meta); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	verdictBuf := make([]byte, 1)
	if _, err := io.ReadFull(conn, verdictBuf); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return ErrTextUnsupported
		}
		return fmt.Errorf("no reply from peer: %w", err)
	}
	if verdictBuf[0] != ResponseAccept {
		return errors.New("peer rejected the message")
	}
	return nil
}
