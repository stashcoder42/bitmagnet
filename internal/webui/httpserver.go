package webui

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"net/http"
	"strings"

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

	staticFS := http.FS(appRoot)

	serveInjectedIndex := func(c *gin.Context) {
		keyInfo, keyErr := apiKeySvc.GetKey(context.Background())
		if keyErr != nil {
			c.String(http.StatusInternalServerError, "failed to get API key")
			return
		}
		configScript := `<script>window.__BITMAGNET_CONFIG__={apiKey:"` + keyInfo.Key + `"};</script>`
		injectedHTML := bytes.Replace(
			originalIndex,
			[]byte("</head>"),
			[]byte(configScript+"</head>"),
			1,
		)
		c.Data(http.StatusOK, "text/html; charset=utf-8", injectedHTML)
	}

	// Serve static files under /webui, with SPA fallback to injected index.html
	e.GET("/webui/*filepath", func(c *gin.Context) {
		filepath := c.Param("filepath")

		// Serve injected index.html for root or index.html
		if filepath == "/" || filepath == "/index.html" || filepath == "" {
			serveInjectedIndex(c)
			return
		}

		// Try to serve the static file
		f, err := staticFS.Open(filepath)
		if err != nil {
			// SPA fallback: serve injected index.html for unknown paths
			serveInjectedIndex(c)
			return
		}
		defer f.Close()

		stat, err := f.Stat()
		if err != nil || stat.IsDir() {
			serveInjectedIndex(c)
			return
		}

		// Determine content type from extension
		contentType := "application/octet-stream"
		if strings.HasSuffix(filepath, ".js") {
			contentType = "application/javascript"
		} else if strings.HasSuffix(filepath, ".css") {
			contentType = "text/css"
		} else if strings.HasSuffix(filepath, ".html") {
			contentType = "text/html; charset=utf-8"
		} else if strings.HasSuffix(filepath, ".json") {
			contentType = "application/json"
		} else if strings.HasSuffix(filepath, ".svg") {
			contentType = "image/svg+xml"
		} else if strings.HasSuffix(filepath, ".woff") {
			contentType = "font/woff"
		} else if strings.HasSuffix(filepath, ".woff2") {
			contentType = "font/woff2"
		} else if strings.HasSuffix(filepath, ".png") {
			contentType = "image/png"
		} else if strings.HasSuffix(filepath, ".ico") {
			contentType = "image/x-icon"
		}

		http.ServeContent(c.Writer, c.Request, stat.Name(), stat.ModTime(), f)
		c.Header("Content-Type", contentType)
	})

	e.GET("/", func(c *gin.Context) {
		c.Redirect(301, "/webui")
	})

	return nil
}
