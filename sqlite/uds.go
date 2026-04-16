package sqliteembed

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// UDS wire format (both directions framed; big-endian uint32 length):
//
//	request:  [4B len][SQL bytes]
//	response: [1B status][4B len][body]
//	          status 0 = ok (body is sqlite3 CSV output)
//	          status 1 = err (body is an error message)
//
// The frame length cap exists purely to bound the daemon's read buffer
// when a peer sends garbage; 64MiB is well above any realistic SQL
// payload or CSV result set but small enough to reject runaway framing.
const maxUDSFrameBytes = 64 << 20

const (
	udsStatusOK  byte = 0
	udsStatusErr byte = 1
)

// writeUDSFrame writes [4B len][payload].
func writeUDSFrame(w io.Writer, payload []byte) error {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(payload)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

// readUDSFrame reads [4B len][payload].
func readUDSFrame(r io.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n > maxUDSFrameBytes {
		return nil, fmt.Errorf("uds frame too large: %d bytes", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// writeUDSResponse writes [status][4B len][body].
func writeUDSResponse(w io.Writer, status byte, body []byte) error {
	if _, err := w.Write([]byte{status}); err != nil {
		return err
	}
	return writeUDSFrame(w, body)
}

// readUDSResponse reads [status][4B len][body].
func readUDSResponse(r io.Reader) (status byte, body []byte, err error) {
	var hdr [1]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, nil, err
	}
	body, err = readUDSFrame(r)
	return hdr[0], body, err
}

// dialUDS opens a connection to the daemon, honoring ctx.Deadline.
func dialUDS(ctx context.Context, socketURI string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, "unix", socketURI)
}

// queryOverUDS dials socketURI, writes sql as one request frame, reads
// one response frame, and returns the CSV body. A non-OK status turns
// into an error carrying the server-side message.
func queryOverUDS(ctx context.Context, socketURI, sql string) (string, error) {
	conn, err := dialUDS(ctx, socketURI)
	if err != nil {
		return "", fmt.Errorf("dial uds %q: %w", socketURI, err)
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	if err := writeUDSFrame(conn, []byte(sql)); err != nil {
		return "", fmt.Errorf("write sql: %w", err)
	}
	status, body, err := readUDSResponse(conn)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if status != udsStatusOK {
		return "", errors.New(strings.TrimSpace(string(body)))
	}
	return string(body), nil
}

// ServeUDS runs the sqlited loop on lis: accept a connection, read one
// SQL frame, execute it against (bin, db) using the sqlite3 CLI, write
// one response frame, close. Returns when lis is closed or ctx is done.
//
// Each connection handles exactly one query. This keeps the daemon
// stateless and sidesteps the session-boundary problem of multiplexing
// arbitrary sqlite statements over one long-lived CLI stdio pair.
func ServeUDS(ctx context.Context, lis net.Listener, bin, db string) error {
	var wg sync.WaitGroup
	go func() {
		<-ctx.Done()
		_ = lis.Close()
	}()
	for {
		conn, err := lis.Accept()
		if err != nil {
			if ctx.Err() != nil {
				wg.Wait()
				return nil
			}
			if errors.Is(err, net.ErrClosed) {
				wg.Wait()
				return nil
			}
			return err
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer conn.Close()
			handleUDSConn(ctx, conn, bin, db)
		}()
	}
}

func handleUDSConn(ctx context.Context, conn net.Conn, bin, db string) {
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	sql, err := readUDSFrame(conn)
	if err != nil {
		_ = writeUDSResponse(conn, udsStatusErr, []byte(fmt.Sprintf("read sql: %v", err)))
		return
	}
	out, err := exec.CommandContext(ctx, bin, "-cmd", ".headers on", "-cmd", ".mode quote", db, string(sql)).CombinedOutput()
	if err != nil {
		msg := fmt.Sprintf("sqlite3: %v (out=%q)", err, string(out))
		_ = writeUDSResponse(conn, udsStatusErr, []byte(msg))
		return
	}
	_ = writeUDSResponse(conn, udsStatusOK, out)
}
