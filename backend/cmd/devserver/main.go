package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

type state int

const (
	stateBuilding state = iota
	stateRunning
	stateError
)

type devServer struct {
	port         int
	internalPort int
	backendDir   string
	binDir       string

	mu        sync.RWMutex
	current   state
	buildErr  string
	proxy     *httputil.ReverseProxy
	childCmd  *exec.Cmd
	childDone chan struct{} // closed when the current child process exits
}

func main() {
	port := flag.Int("port", 8080, "External HTTP port")
	flag.Parse()

	internalPort, err := findFreePort()
	if err != nil {
		log.Fatalf("Failed to find free port: %v", err)
	}

	proxyURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", internalPort))

	s := &devServer{
		port:         *port,
		internalPort: internalPort,
		backendDir:   "backend",
		binDir:       "bin",
		proxy:        httputil.NewSingleHostReverseProxy(proxyURL),
	}

	// Suppress proxy error logging for connection-refused (child not ready yet).
	s.proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		s.mu.RLock()
		st := s.current
		s.mu.RUnlock()
		if st == stateBuilding {
			jsonError(w, "Backend is rebuilding, please wait...", http.StatusServiceUnavailable)
		} else {
			jsonError(w, fmt.Sprintf("Backend proxy error: %v", err), http.StatusBadGateway)
		}
	}

	go s.handleSignals()

	// Initial build is synchronous so the HTTP listener only starts once the
	// backend is ready (or has failed). This preserves the port-wait semantics
	// that init.sh relies on.
	s.buildAndRestart()

	buildCh := make(chan struct{}, 1)
	go s.builder(buildCh)
	go s.watchFiles(buildCh)

	log.Printf("Dev server listening on :%d (proxying to :%d)", s.port, s.internalPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", s.port), s))
}

func findFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}

func (s *devServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	st := s.current
	buildErr := s.buildErr
	s.mu.RUnlock()

	switch st {
	case stateRunning, stateBuilding:
		s.proxy.ServeHTTP(w, r)
	case stateError:
		jsonError(w, buildErr, http.StatusInternalServerError)
	}
}

func (s *devServer) watchFiles(buildCh chan<- struct{}) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Failed to create file watcher: %v", err)
	}
	defer watcher.Close()

	// Walk and add all directories under backendDir.
	err = filepath.Walk(s.backendDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip hidden directories and testdata.
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "testdata" || name == "vendor" {
				return filepath.SkipDir
			}
			return watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Failed to set up file watches: %v", err)
	}

	var debounce *time.Timer

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			if !isRelevantFile(event.Name) {
				// If a new directory was created, watch it.
				if event.Has(fsnotify.Create) {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						watcher.Add(event.Name)
					}
				}
				continue
			}

			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(500*time.Millisecond, func() {
				// Non-blocking send: if a build is already queued, skip.
				select {
				case buildCh <- struct{}{}:
				default:
				}
			})

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

func (s *devServer) builder(buildCh <-chan struct{}) {
	for range buildCh {
		s.buildAndRestart()
	}
}

func (s *devServer) buildAndRestart() {
	s.mu.Lock()
	s.current = stateBuilding
	s.mu.Unlock()

	log.Println("Building backend...")

	tmpBin := filepath.Join(s.binDir, "backend-tmp")
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", filepath.Join("..", tmpBin), ".")
	cmd.Dir = s.backendDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		buildOutput := strings.TrimSpace(string(output))
		if buildOutput == "" {
			buildOutput = err.Error()
		}
		log.Printf("Build failed:\n%s", buildOutput)

		s.mu.Lock()
		s.stopChildLocked()
		s.current = stateError
		s.buildErr = buildOutput
		s.mu.Unlock()

		os.Remove(tmpBin)
		return
	}

	// Build succeeded: swap binary.
	finalBin := filepath.Join(s.binDir, "backend")
	if err := os.Rename(tmpBin, finalBin); err != nil {
		log.Printf("Failed to rename binary: %v", err)
		s.mu.Lock()
		s.current = stateError
		s.buildErr = fmt.Sprintf("Failed to rename binary: %v", err)
		s.mu.Unlock()
		return
	}

	s.mu.Lock()
	s.stopChildLocked()
	s.mu.Unlock()

	if err := s.startChild(); err != nil {
		log.Printf("Failed to start backend: %v", err)
		s.mu.Lock()
		s.current = stateError
		s.buildErr = fmt.Sprintf("Failed to start backend: %v", err)
		s.mu.Unlock()
		return
	}

	s.mu.Lock()
	s.current = stateRunning
	s.buildErr = ""
	s.mu.Unlock()

	log.Println("Backend started successfully.")
}

func (s *devServer) startChild() error {
	bin := filepath.Join(s.binDir, "backend")
	cmd := exec.Command("./"+bin, "--http-port", fmt.Sprintf("%d", s.internalPort))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("exec: %w", err)
	}

	done := make(chan struct{})

	// Start the monitor goroutine immediately so that done is always closed
	// when the process exits. This prevents a deadlock in stopChildLocked
	// if waitForPort fails before the goroutine would otherwise be launched.
	go func() {
		err := cmd.Wait()
		close(done)

		s.mu.Lock()
		defer s.mu.Unlock()

		// Only react if this is still the current child.
		if s.childCmd != nil && s.childCmd.Process != nil && s.childCmd.Process.Pid == cmd.Process.Pid {
			s.childCmd = nil
			if s.current == stateRunning {
				exitMsg := "Backend exited unexpectedly"
				if err != nil {
					exitMsg = fmt.Sprintf("Backend exited: %v", err)
				}
				log.Println(exitMsg)
				s.current = stateError
				s.buildErr = exitMsg
			}
		}
	}()

	s.mu.Lock()
	s.childCmd = cmd
	s.childDone = done
	s.mu.Unlock()

	if err := s.waitForPort(s.internalPort, 10*time.Second); err != nil {
		s.mu.Lock()
		s.stopChildLocked()
		s.mu.Unlock()
		return fmt.Errorf("backend did not start: %w", err)
	}

	return nil
}

func (s *devServer) stopChildLocked() {
	if s.childCmd == nil || s.childCmd.Process == nil {
		return
	}

	pid := s.childCmd.Process.Pid
	done := s.childDone

	// Send SIGTERM to the process group.
	syscall.Kill(-pid, syscall.SIGTERM)

	// Wait up to 5 seconds for the monitor goroutine to reap the process.
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		syscall.Kill(-pid, syscall.SIGKILL)
		<-done
	}

	s.childCmd = nil
}

func (s *devServer) waitForPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("port %d not reachable within %v", port, timeout)
}

func (s *devServer) handleSignals() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	sig := <-sigCh

	log.Printf("Received %v, shutting down...", sig)

	s.mu.Lock()
	s.stopChildLocked()
	s.mu.Unlock()

	os.Exit(0)
}

func isRelevantFile(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".go" || ext == ".mod" || ext == ".sum"
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"message": msg})
}
