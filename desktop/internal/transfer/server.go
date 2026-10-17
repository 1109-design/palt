package transfer

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"palt/internal/secure"
)

// Server handles incoming PALT TCP connections and file downloads.
type Server struct {
	port     int
	listener net.Listener
	running  atomic.Bool

	// OnOffer blockingly decides whether to accept an incoming file.
	OnOffer func(meta Metadata) (accept bool)

	// OnProgress provides real-time chunk progress back to the UI.
	OnProgress func(meta Metadata, written int64, total int64, sentItems int, totalItems int, currentFile string)

	// OnText is called when a text message arrives. Messages are delivered
	// without an accept prompt.
	OnText func(meta Metadata)

	// OnComplete is called when a transfer finishes successfully.
	// savedPaths[i] is where meta.Files[i] was written.
	OnComplete func(meta Metadata, savedPaths []string)

	// OnError is called when a transfer fails. savedPaths covers the files
	// opened before the failure (the last one may be partial).
	OnError func(meta Metadata, savedPaths []string, err error)

	// Keys returns this device's pairing keys; ok is false when unpaired,
	// and v2 (paired) connections are then refused.
	Keys func() (keys secure.Keys, ok bool)

	// OnLink takes over a link connection from a paired device. The handler
	// owns conn.
	OnLink func(conn net.Conn, meta Metadata)
}

// NewServer initializes a TCP receiver.
func NewServer(port int) *Server {
	return &Server{
		port: port,
	}
}

// Start opens the TCP listener loop in a background goroutine.
func (s *Server) Start() error {
	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = l
	s.running.Store(true)

	log.Printf("[TransferServer] Listening for chunks on %s\n", addr)

	go s.acceptLoop()
	return nil
}

// Stop halts the listener.
func (s *Server) Stop() {
	s.running.Store(false)
	if s.listener != nil {
		s.listener.Close()
	}
}

func (s *Server) acceptLoop() {
	for s.running.Load() {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.running.Load() {
				log.Printf("[TransferServer] Accept Error: %v\n", err)
			}
			continue
		}

		go s.HandleConn(conn, RouteDirect)
	}
}

// HandleConn serves one PALT exchange arriving on a LAN socket: a plain v1
// exchange from any device, or a v2 encrypted one from a paired device.
func (s *Server) HandleConn(conn net.Conn, route string) {
	first := make([]byte, 4)
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	if _, err := io.ReadFull(conn, first); err != nil {
		// A bare connect-and-close is a peer checking we're reachable.
		if err != io.EOF {
			log.Printf("[TransferServer] Failed to read offer protocol: %v\n", err)
		}
		conn.Close()
		return
	}

	if string(first) == helloMagic {
		hello, raw, err := readHello(conn)
		if err != nil {
			log.Printf("[TransferServer] Bad hello from %s: %v\n", conn.RemoteAddr(), err)
			conn.Close()
			return
		}
		keys, ok := secure.Keys{}, false
		if s.Keys != nil {
			keys, ok = s.Keys()
		}
		if !ok {
			conn.Close()
			return
		}
		sc, err := secure.Handshake(conn, keys, helloSession(raw), false)
		if err != nil {
			log.Printf("[TransferServer] Handshake with %s failed: %v\n", conn.RemoteAddr(), err)
			conn.Close()
			return
		}
		s.HandlePaired(sc, route, hello.From)
		return
	}

	meta, err := readOfferBody(conn, first)
	conn.SetReadDeadline(time.Time{})
	if err != nil {
		log.Printf("[TransferServer] Failed to read offer protocol: %v\n", err)
		conn.Close()
		return
	}
	meta.Route = route
	meta.Paired, meta.SenderID = false, ""
	s.serve(conn, meta)
}

// HandlePaired serves an encrypted connection from a paired device: a v2 LAN
// connection after its handshake, or a relay pipe.
func (s *Server) HandlePaired(conn net.Conn, route, from string) {
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	meta, err := ReadOffer(conn)
	conn.SetReadDeadline(time.Time{})
	if err != nil {
		if err != io.EOF {
			log.Printf("[TransferServer] Failed to read offer from paired device: %v\n", err)
		}
		conn.Close()
		return
	}
	meta.Route = route
	meta.Paired, meta.SenderID = true, from

	if meta.Action == ActionLink {
		if s.OnLink == nil {
			conn.Close()
			return
		}
		if _, err := conn.Write([]byte{ResponseAccept}); err != nil {
			conn.Close()
			return
		}
		s.OnLink(conn, *meta)
		return
	}
	s.serve(conn, meta)
}

// serve runs a text or file exchange whose offer has been read.
func (s *Server) serve(conn net.Conn, meta *Metadata) {
	defer conn.Close()

	if meta.Action == ActionText {
		if len(meta.Text) == 0 || len(meta.Text) > MaxTextBytes {
			conn.Write([]byte{ResponseReject})
			return
		}
		if _, err := conn.Write([]byte{ResponseAccept}); err != nil {
			log.Printf("[TransferServer] Failed to ack text message: %v\n", err)
			return
		}
		if s.OnText != nil {
			s.OnText(*meta)
		}
		return
	}

	if meta.Action != ActionOffer {
		log.Printf("[TransferServer] Unknown action: %v\n", meta.Action)
		return
	}

	// 2. Surface to UI and Wait for Verdict (blocking)
	var accept bool

	if s.OnOffer != nil {
		accept = s.OnOffer(*meta)
	} else {
		accept = false
	}

	// 3. Reject Route
	if !accept {
		conn.Write([]byte{ResponseReject})
		return
	}

	// 4. Accept Route
	_, err := conn.Write([]byte{ResponseAccept})
	if err != nil {
		log.Printf("[TransferServer] Failed to send Accept byte: %v\n", err)
		return
	}

	// 5. Open Default Hardware PALT Directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[TransferServer] Could not find user home for saving: %v\n", err)
		return
	}
	downloadDir := filepath.Join(homeDir, "Downloads", "PALT")
	os.MkdirAll(downloadDir, os.ModePerm)

	// 6. IO Chunk Copy Loop
	var totalWritten int64 = 0
	totalFiles := len(meta.Files)
	savedPaths := make([]string, 0, totalFiles)

	for i, f := range meta.Files {
		savePath := resolveUniqueFilePath(downloadDir, safeFileName(f.Name))
		savedPaths = append(savedPaths, savePath)
		file, err := os.Create(savePath)
		if err != nil {
			log.Printf("[TransferServer] Failed to create output file %s: %v\n", savePath, err)
			return
		}

		tracker := &TrackingWriter{
			Out:           file,
			Total:         meta.TotalSize,
			Written:       totalWritten,
			BroadcastStep: meta.TotalSize / 100,
			OnProgress: func(written int64, total int64) {
				if s.OnProgress != nil {
					s.OnProgress(*meta, written, total, i+1, totalFiles, f.Name)
				}
			},
		}

		if tracker.BroadcastStep == 0 {
			tracker.BroadcastStep = 1024 * 1024 // Fallback 1MB if too small
		}

		limitReader := io.LimitReader(conn, f.Size)
		buf := make([]byte, 1024*1024) // 1MB buffer
		copied, err := io.CopyBuffer(tracker, limitReader, buf)
		file.Close()
		totalWritten += copied

		if err != nil && err != io.EOF {
			log.Printf("[TransferServer] Interrupted mid-transfer on file %s: only read %d/%d bytes: %v\n", f.Name, copied, f.Size, err)
			if s.OnError != nil {
				s.OnError(*meta, savedPaths, err)
			}
			return
		}

		log.Printf("[TransferServer] Streamed %s completely (%d bytes).\n", f.Name, copied)
	}

	log.Printf("[TransferServer] Transfer %s completely finished across %d files.\n", meta.TransferID, totalFiles)
	if s.OnComplete != nil {
		s.OnComplete(*meta, savedPaths)
	}
}

// safeFileName reduces a peer-supplied name to a bare file name so it can't
// escape the download directory (e.g. "../../.bashrc" or "/etc/passwd").
func safeFileName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	if name == "." || name == ".." || name == "/" || name == "" {
		return "file"
	}
	return name
}

// resolveUniqueFilePath generates a non-colliding filename by appending (1), (2), etc.
func resolveUniqueFilePath(dir, name string) string {
	ext := filepath.Ext(name)
	base := name[:len(name)-len(ext)]

	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}

	for i := 1; ; i++ {
		newName := fmt.Sprintf("%s (%d)%s", base, i, ext)
		path = filepath.Join(dir, newName)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path
		}
	}
}
