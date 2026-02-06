package httpserver

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/bitmagnet-io/bitmagnet/internal/httpserver"
	"github.com/bitmagnet-io/bitmagnet/internal/lazy"
	"github.com/gin-gonic/gin"
	"github.com/vektah/gqlparser/v2/ast"
	"go.uber.org/fx"
)

type Params struct {
	fx.In
	Schema lazy.Lazy[graphql.ExecutableSchema]
	Config httpserver.Config
}

type Result struct {
	fx.Out
	Option httpserver.Option `group:"http_server_options"`
}

func New(p Params) Result {
	return Result{
		Option: &builder{
			schema: p.Schema,
			apiKey: p.Config.APIKey,
		},
	}
}

type builder struct {
	schema lazy.Lazy[graphql.ExecutableSchema]
	apiKey string
}

func (builder) Key() string {
	return "graphql"
}

func (b builder) Apply(e *gin.Engine) error {
	schema, err := b.schema.Get()
	if err != nil {
		return err
	}

	gql := newServer(schema)
	pg := playground.Handler("GraphQL playground", "/graphql")

	if b.apiKey == "" {
		e.POST("/graphql", func(c *gin.Context) {
			gql.ServeHTTP(c.Writer, c.Request)
		})
		e.GET("/graphql", func(c *gin.Context) {
			pg.ServeHTTP(c.Writer, c.Request)
		})
	} else {
		authMiddleware := createAuthMiddleware(b.apiKey)
		e.POST("/graphql", authMiddleware, func(c *gin.Context) {
			gql.ServeHTTP(c.Writer, c.Request)
		})
		e.GET("/graphql", authMiddleware, func(c *gin.Context) {
			pg.ServeHTTP(c.Writer, c.Request)
		})
	}

	return nil
}

func createAuthMiddleware(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing authorization header",
				"code":  "UNAUTHORIZED",
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid authorization header format",
				"code":  "UNAUTHORIZED",
			})
			return
		}

		token := parts[1]
		if subtle.ConstantTimeCompare([]byte(token), []byte(apiKey)) != 1 {
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
