package httpserver

import (
	"net/http"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/bitmagnet-io/bitmagnet/internal/apikey"
	"github.com/bitmagnet-io/bitmagnet/internal/httpserver"
	"github.com/bitmagnet-io/bitmagnet/internal/lazy"
	"github.com/gin-gonic/gin"
	"github.com/vektah/gqlparser/v2/ast"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type Params struct {
	fx.In
	Schema        lazy.Lazy[graphql.ExecutableSchema]
	APIKeyService lazy.Lazy[apikey.Service]
	Logger        *zap.SugaredLogger
}

type Result struct {
	fx.Out
	Option httpserver.Option `group:"http_server_options"`
}

func New(p Params) Result {
	return Result{
		Option: &builder{
			schema:        p.Schema,
			apiKeyService: p.APIKeyService,
		},
	}
}

type builder struct {
	schema        lazy.Lazy[graphql.ExecutableSchema]
	apiKeyService lazy.Lazy[apikey.Service]
}

func (builder) Key() string {
	return "graphql"
}

func (b builder) Apply(e *gin.Engine) error {
	schema, err := b.schema.Get()
	if err != nil {
		return err
	}

	apiKeySvc, err := b.apiKeyService.Get()
	if err != nil {
		return err
	}

	gql := newServer(schema)

	authMiddleware := createAuthMiddleware(apiKeySvc)

	e.POST("/graphql", authMiddleware, func(c *gin.Context) {
		gql.ServeHTTP(c.Writer, c.Request)
	})

	pg := playground.Handler("GraphQL playground", "/graphql")

	e.GET("/graphql", authMiddleware, func(c *gin.Context) {
		pg.ServeHTTP(c.Writer, c.Request)
	})

	return nil
}

func createAuthMiddleware(apiKeySvc apikey.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing authorization header",
				"code":  "UNAUTHORIZED",
			})
			return
		}

		// Expect "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid authorization header format",
				"code":  "UNAUTHORIZED",
			})
			return
		}

		token := parts[1]
		valid, err := apiKeySvc.ValidateKey(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "failed to validate API key",
				"code":  "INTERNAL_ERROR",
			})
			return
		}

		if !valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid API key",
				"code":  "UNAUTHORIZED",
			})
			return
		}

		c.Next()
	}
}

func newServer(es graphql.ExecutableSchema) *handler.Server {
	srv := handler.New(es)

	srv.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
	})
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.MultipartForm{})

	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))

	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

	return srv
}
