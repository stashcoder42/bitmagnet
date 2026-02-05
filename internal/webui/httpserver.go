package webui

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"time"

	"github.com/bitmagnet-io/bitmagnet/internal/apikey"
	"github.com/bitmagnet-io/bitmagnet/internal/httpserver"
	"github.com/bitmagnet-io/bitmagnet/internal/lazy"
	"github.com/bitmagnet-io/bitmagnet/webui"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type Params struct {
	fx.In
	Logger        *zap.SugaredLogger
	APIKeyService lazy.Lazy[apikey.Service]
}

type Result struct {
	fx.Out
	Option httpserver.Option `group:"http_server_options"`
}

func New(p Params) Result {
	return Result{
		Option: &builder{
			logger:        p.Logger.Named("webui"),
			apiKeyService: p.APIKeyService,
		},
	}
}

type builder struct {
	logger        *zap.SugaredLogger
	apiKeyService lazy.Lazy[apikey.Service]
}

func (*builder) Key() string {
	return "webui"
}

func (b *builder) Apply(e *gin.Engine) error {
	webuiFS := webui.StaticFS()

	appRoot, appRootErr := fs.Sub(webuiFS, "dist/bitmagnet/browser")
	if appRootErr != nil {
		b.logger.Errorf(
			"the webui app root directory is missing; run `npm run build` within the `webui` folder: %v",
			appRootErr)

		return nil
	}

	apiKeySvc, err := b.apiKeyService.Get()
	if err != nil {
		return err
	}

	// Read the original index.html at startup
	indexFile, err := appRoot.Open("index.html")
	if err != nil {
		b.logger.Errorf("failed to open index.html: %v", err)
		return nil
	}
	defer indexFile.Close()

	originalIndex, err := io.ReadAll(indexFile)
	if err != nil {
		b.logger.Errorf("failed to read index.html: %v", err)
		return nil
	}

	// Create wrapped filesystem that injects API key into index.html
	wrappedFS := &apiKeyInjectingFS{
		FileSystem:    http.FS(appRoot),
		apiKeyService: apiKeySvc,
		originalIndex: originalIndex,
	}

	e.StaticFS("/webui", wrappedFS)
	e.GET("/", func(c *gin.Context) {
		c.Redirect(301, "/webui")
	})

	return nil
}

type apiKeyInjectingFS struct {
	http.FileSystem
	apiKeyService apikey.Service
	originalIndex []byte
}

func (fs *apiKeyInjectingFS) Open(name string) (http.File, error) {
	// Serve injected index.html for root path, index.html requests, or SPA fallback
	if name == "/" || name == "/index.html" {
		return fs.serveInjectedIndex()
	}

	f, err := fs.FileSystem.Open(name)
	if err != nil && errors.Is(err, io.EOF) {
		return nil, err
	}
	if err != nil {
		// SPA routing fallback - serve index.html for missing files
		return fs.serveInjectedIndex()
	}

	return f, nil
}

func (fs *apiKeyInjectingFS) serveInjectedIndex() (http.File, error) {
	// Get the current API key
	keyInfo, err := fs.apiKeyService.GetKey(context.Background())
	if err != nil {
		return nil, err
	}

	// Inject the API key config script before </head>
	configScript := `<script>window.__BITMAGNET_CONFIG__={apiKey:"` + keyInfo.Key + `"};</script>`
	injectedHTML := bytes.Replace(
		fs.originalIndex,
		[]byte("</head>"),
		[]byte(configScript+"</head>"),
		1,
	)

	return &inMemoryFile{
		Reader: bytes.NewReader(injectedHTML),
		name:   "index.html",
		size:   int64(len(injectedHTML)),
	}, nil
}

// inMemoryFile implements http.File for in-memory content
type inMemoryFile struct {
	*bytes.Reader
	name string
	size int64
}

func (f *inMemoryFile) Close() error {
	return nil
}

func (f *inMemoryFile) Readdir(count int) ([]fs.FileInfo, error) {
	return nil, nil
}

func (f *inMemoryFile) Stat() (fs.FileInfo, error) {
	return &inMemoryFileInfo{name: f.name, size: f.size}, nil
}

type inMemoryFileInfo struct {
	name string
	size int64
}

func (fi *inMemoryFileInfo) Name() string       { return fi.name }
func (fi *inMemoryFileInfo) Size() int64        { return fi.size }
func (fi *inMemoryFileInfo) Mode() fs.FileMode  { return 0444 }
func (fi *inMemoryFileInfo) ModTime() time.Time { return time.Now() }
func (fi *inMemoryFileInfo) IsDir() bool        { return false }
func (fi *inMemoryFileInfo) Sys() interface{}   { return nil }
