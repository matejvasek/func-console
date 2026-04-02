package main

import (
	"crypto/tls"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/ory/viper"
	"knative.dev/func/cmd/ci"
	"knative.dev/func/pkg/functions"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	httpPort := flag.Int("http-port", 8080, "HTTP server port")
	httpsPort := flag.Int("https-port", 8443, "HTTPS server port")
	certFile := flag.String("cert", "/var/cert/tls.crt", "TLS certificate file")
	keyFile := flag.String("key", "/var/cert/tls.key", "TLS key file")
	flag.Parse()

	static, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to create sub filesystem: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/function/create", handleFuncCreate)
	mux.Handle("/", http.FileServer(http.FS(static)))

	handler := loggingMiddleware(mux)

	_, certErr := os.Stat(*certFile)
	_, keyErr := os.Stat(*keyFile)
	if certErr == nil && keyErr == nil {
		go func() {
			ln, err := net.Listen("tcp", fmt.Sprintf(":%d", *httpPort))
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("Listening on http://%s", ln.Addr())
			log.Fatal(http.Serve(ln, handler))
		}()

		cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			log.Fatalf("Failed to load TLS certificate: %v", err)
		}
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", *httpsPort))
		if err != nil {
			log.Fatal(err)
		}
		tlsLn := tls.NewListener(ln, &tls.Config{
			Certificates: []tls.Certificate{cert},
		})
		log.Printf("Listening on https://%s", ln.Addr())
		log.Fatal(http.Serve(tlsLn, handler))
	} else {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", *httpPort))
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("TLS certificate not found, listening on http://%s", ln.Addr())
		log.Fatal(http.Serve(ln, handler))
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

type funcCreateRequest struct {
	Name      string `json:"name"`
	Runtime   string `json:"runtime"`
	Registry  string `json:"registry"`
	Namespace string `json:"namespace"`
	Branch    string `json:"branch"`
}

// validName restricts function names to lowercase DNS-label characters.
var validName = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// validRuntimes is the set of supported function runtimes.
var validRuntimes = map[string]bool{
	"node": true, "python": true, "go": true, "quarkus": true,
}

// validBranch restricts branch names to safe git ref characters.
var validBranch = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._/-]*[a-zA-Z0-9])?$`)

// validNamespace restricts namespaces to valid Kubernetes names.
var validNamespace = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// ciMu serializes access to viper globals used by ci.NewCIConfig.
var ciMu sync.Mutex

type fileEntry struct {
	Path    string `json:"path"`
	Mode    string `json:"mode"`
	Content string `json:"content"`
	Type    string `json:"type"`
}

func handleFuncCreate(w http.ResponseWriter, r *http.Request) {
	var cfg funcCreateRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if !validName.MatchString(cfg.Name) {
		http.Error(w, "invalid function name: must contain only lowercase alphanumeric characters and hyphens", http.StatusBadRequest)
		return
	}
	if !validRuntimes[cfg.Runtime] {
		http.Error(w, "invalid runtime: must be one of node, python, go, quarkus", http.StatusBadRequest)
		return
	}
	if !validBranch.MatchString(cfg.Branch) {
		http.Error(w, "invalid branch name", http.StatusBadRequest)
		return
	}
	if !validNamespace.MatchString(cfg.Namespace) {
		http.Error(w, "invalid namespace: must contain only lowercase alphanumeric characters and hyphens", http.StatusBadRequest)
		return
	}

	tmpDir, err := os.MkdirTemp("", "func-create-*")
	if err != nil {
		http.Error(w, "failed to create temp dir: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)

	root := filepath.Join(tmpDir, cfg.Name)

	client := functions.New()
	_, err = client.Init(functions.Function{
		Name:      cfg.Name,
		Root:      root,
		Runtime:   cfg.Runtime,
		Registry:  cfg.Registry,
		Namespace: cfg.Namespace,
		Template:  "http",
	})
	if err != nil {
		http.Error(w, "failed to initialize function: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := generateCIWorkflow(root, cfg.Branch); err != nil {
		http.Error(w, "failed to generate CI workflow: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var files []fileEntry
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		mode := "100644"
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&0111 != 0 {
			mode = "100755"
		}
		if info.Mode()&os.ModeSymlink != 0 {
			mode = "120000"
		}
		files = append(files, fileEntry{
			Path:    relPath,
			Mode:    mode,
			Content: string(content),
			Type:    "blob",
		})
		return nil
	})
	if err != nil {
		http.Error(w, "failed to read generated files: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(files); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}

func generateCIWorkflow(root, branch string) error {
	ciMu.Lock()
	defer ciMu.Unlock()

	viper.Set(ci.PlatformFlag, ci.DefaultPlatform)
	viper.Set(ci.PathFlag, root)
	viper.Set(ci.BranchFlag, branch)
	viper.Set(ci.WorkflowNameFlag, ci.DefaultWorkflowName)
	viper.Set(ci.KubeconfigSecretNameFlag, ci.DefaultKubeconfigSecretName)
	viper.Set(ci.RegistryLoginUrlVariableNameFlag, ci.DefaultRegistryLoginUrlVariableName)
	viper.Set(ci.RegistryUserVariableNameFlag, ci.DefaultRegistryUserVariableName)
	viper.Set(ci.RegistryPassSecretNameFlag, ci.DefaultRegistryPassSecretName)
	viper.Set(ci.RegistryUrlVariableNameFlag, ci.DefaultRegistryUrlVariableName)
	viper.Set(ci.UseRegistryLoginFlag, ci.DefaultUseRegistryLogin)
	viper.Set(ci.WorkflowDispatchFlag, ci.DefaultWorkflowDispatch)
	viper.Set(ci.UseRemoteBuildFlag, ci.DefaultUseRemoteBuild)
	viper.Set(ci.UseSelfHostedRunnerFlag, ci.DefaultUseSelfHostedRunner)

	// Branch is already set via viper, so currentBranch won't be called.
	noop := func(string) (string, error) { return "", nil }
	workDir := func() (string, error) { return root, nil }

	cfg, err := ci.NewCIConfig(noop, workDir)
	if err != nil {
		return err
	}

	workflow := ci.NewGitHubWorkflow(cfg)
	return workflow.Export(cfg.FnGitHubWorkflowFilepath(root), ci.DefaultWorkflowWriter)
}
